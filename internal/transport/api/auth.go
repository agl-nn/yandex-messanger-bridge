package api

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	//"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"yandex-messenger-bridge/internal/repository/interface"
)

type AuthAPI struct {
	repo      _interface.IntegrationRepository
	jwtSecret []byte
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token              string `json:"token,omitempty"`
	MustChangePassword bool   `json:"must_change_password,omitempty"`
	Message            string `json:"message,omitempty"`
	User               struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user,omitempty"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func NewAuthAPI(repo _interface.IntegrationRepository, jwtSecret string) *AuthAPI {
	return &AuthAPI{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
	}
}

func (a *AuthAPI) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind login request")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	log.Info().Str("email", req.Email).Msg("Login attempt")

	user, err := a.repo.FindUserByEmail(c.Request().Context(), req.Email)
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("User not found")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Error().Err(err).Msg("Password mismatch")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	// Проверяем, нужно ли сменить пароль
	if user.MustChangePassword {
		log.Info().Str("user_id", user.ID).Msg("User must change password")

		// Создаем временный токен для смены пароля
		tempToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":     user.ID,
			"role":        user.Role,
			"must_change": true,
			"exp":         time.Now().Add(1 * time.Hour).Unix(), // Короткий срок
		})

		tempTokenString, err := tempToken.SignedString(a.jwtSecret)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate temp token")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		}

		// Устанавливаем временную cookie
		c.SetCookie(&http.Cookie{
			Name:     "temp_token",
			Value:    tempTokenString,
			Path:     "/",
			Expires:  time.Now().Add(1 * time.Hour),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		return c.JSON(http.StatusOK, LoginResponse{
			MustChangePassword: true,
			Message:            "Please change your password",
		})
	}

	// Обычная авторизация
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}

	// Устанавливаем основную cookie
	c.SetCookie(&http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	response := LoginResponse{
		Token: tokenString,
		User: struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		}{
			ID:    user.ID,
			Email: user.Email,
			Role:  user.Role,
		},
	}

	log.Info().Str("user_id", user.ID).Msg("Login successful")
	return c.JSON(http.StatusOK, response)
}

func (a *AuthAPI) Me(c echo.Context) error {
	userID := c.Get("user_id").(string)

	user, err := a.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":                   user.ID,
		"email":                user.Email,
		"role":                 user.Role,
		"must_change_password": user.MustChangePassword,
	})
}

// ChangePassword обрабатывает смену пароля
func (a *AuthAPI) ChangePassword(c echo.Context) error {
	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Пробуем получить user_id из обычного токена или временного
	userID := c.Get("user_id")
	if userID == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	uid := userID.(string)

	user, err := a.repo.FindUserByID(c.Request().Context(), uid)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	// Проверяем текущий пароль
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword))
	if err != nil {
		log.Error().Err(err).Msg("Current password mismatch")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid current password"})
	}

	// Хешируем новый пароль
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash new password")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to change password"})
	}

	// Сохраняем новый пароль и сбрасываем флаг must_change
	if err := a.repo.ChangePassword(c.Request().Context(), uid, string(newHash)); err != nil {
		log.Error().Err(err).Msg("Failed to change password")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to change password"})
	}

	// Удаляем временную cookie если была
	c.SetCookie(&http.Cookie{
		Name:     "temp_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-24 * time.Hour),
		HttpOnly: true,
	})

	log.Info().Str("user_id", uid).Msg("Password changed successfully")
	return c.JSON(http.StatusOK, map[string]string{"message": "password changed successfully"})
}

// Logout обрабатывает выход
func (a *AuthAPI) Logout(c echo.Context) error {
	// Удаляем все cookie
	c.SetCookie(&http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-24 * time.Hour),
		HttpOnly: true,
	})

	c.SetCookie(&http.Cookie{
		Name:     "temp_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-24 * time.Hour),
		HttpOnly: true,
	})

	return c.NoContent(http.StatusOK)
}
