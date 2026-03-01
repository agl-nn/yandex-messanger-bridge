package api

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
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
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
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

	log.Info().Str("user_id", user.ID).Str("email", user.Email).Msg("User found")

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Error().Err(err).Msg("Password mismatch")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	log.Info().Msg("Password correct, generating token")

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

	// Устанавливаем cookie (ЭТО ГЛАВНОЕ ИЗМЕНЕНИЕ)
	c.SetCookie(&http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true, // Защита от XSS
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
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}
