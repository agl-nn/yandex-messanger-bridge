package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/domain"
	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/transport/middleware"
)

// AuthAPI - обработчик аутентификации
type AuthAPI struct {
	repo           _interface.IntegrationRepository
	authMiddleware *middleware.AuthMiddleware
}

// NewAuthAPI создает новый API аутентификации
func NewAuthAPI(repo _interface.IntegrationRepository, authMiddleware *middleware.AuthMiddleware) *AuthAPI {
	return &AuthAPI{
		repo:           repo,
		authMiddleware: authMiddleware,
	}
}

// LoginRequest - запрос на вход
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login обрабатывает вход пользователя
func (a *AuthAPI) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Ищем пользователя
	user, err := a.repo.FindUserByEmail(c.Request().Context(), req.Email)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	// Проверяем пароль
	if !middleware.CheckPassword(req.Password, user.PasswordHash) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	// Генерируем JWT
	token, err := a.authMiddleware.GenerateJWT(user.ID, user.Role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

// RegisterRequest - запрос на регистрацию
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// Register регистрирует нового пользователя
func (a *AuthAPI) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Хешируем пароль
	hash, err := middleware.HashPassword(req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
	}

	// Создаем пользователя
	user := &domain.User{
		Email:        req.Email,
		PasswordHash: hash,
		Role:         "user",
	}

	if err := a.repo.CreateUser(c.Request().Context(), user); err != nil {
		// Проверяем уникальность email
		return c.JSON(http.StatusConflict, map[string]string{"error": "email already exists"})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
	})
}

// Me возвращает информацию о текущем пользователе
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
