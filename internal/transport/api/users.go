package api

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"yandex-messenger-bridge/internal/domain"
	"yandex-messenger-bridge/internal/repository/interface"
)

type UsersAPI struct {
	repo      _interface.IntegrationRepository
	jwtSecret []byte
}

type CreateUserRequest struct {
	Email         string `json:"email" validate:"required,email"`
	Role          string `json:"role" validate:"required,oneof=user admin"`
	RequireChange bool   `json:"require_change"`
}

type ResetPasswordResponse struct {
	TempPassword string `json:"temp_password"`
	Message      string `json:"message"`
}

func NewUsersAPI(repo _interface.IntegrationRepository, jwtSecret string) *UsersAPI {
	return &UsersAPI{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
	}
}

// ListUsers возвращает список всех пользователей (только для админов)
func (u *UsersAPI) ListUsers(c echo.Context) error {
	// Проверяем права админа
	userRole := c.Get("user_role").(string)
	if userRole != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}

	users, err := u.repo.ListUsers(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list users")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
	}

	// Скрываем хеши паролей
	for _, user := range users {
		user.PasswordHash = ""
	}

	return c.JSON(http.StatusOK, users)
}

// CreateUser создает нового пользователя (только для админов)
func (u *UsersAPI) CreateUser(c echo.Context) error {
	// Проверяем права админа
	userRole := c.Get("user_role").(string)
	if userRole != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}

	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Проверяем, не существует ли уже пользователь
	existing, _ := u.repo.FindUserByEmail(c.Request().Context(), req.Email)
	if existing != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "user already exists"})
	}

	// Генерируем временный пароль
	tempPassword := generateTempPassword()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
	}

	user := &domain.User{
		Email:              req.Email,
		PasswordHash:       string(hashedPassword),
		Role:               req.Role,
		MustChangePassword: req.RequireChange,
	}

	if err := u.repo.CreateUser(c.Request().Context(), user); err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
	}

	// В реальном проекте здесь должна быть отправка email с временным паролем
	log.Info().Str("email", req.Email).Str("temp_password", tempPassword).Msg("User created")

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message":       "user created successfully",
		"temp_password": tempPassword, // Временное решение, в продакшене убрать!
		"user": map[string]string{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

// ResetPassword сбрасывает пароль пользователя (только для админов)
func (u *UsersAPI) ResetPassword(c echo.Context) error {
	// Проверяем права админа
	userRole := c.Get("user_role").(string)
	if userRole != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}

	userID := c.Param("id")

	// Находим пользователя
	user, err := u.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	// Нельзя сбросить пароль у admin@localhost через API
	if user.Email == "admin@localhost" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot reset password for default admin"})
	}

	// Генерируем временный пароль
	tempPassword := generateTempPassword()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reset password"})
	}

	// Обновляем пароль и устанавливаем флаг смены
	if err := u.repo.ChangePassword(c.Request().Context(), userID, string(hashedPassword)); err != nil {
		log.Error().Err(err).Msg("Failed to change password")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reset password"})
	}

	// Дополнительно устанавливаем флаг must_change_password через UpdateUser
	user.MustChangePassword = true
	if err := u.repo.UpdateUser(c.Request().Context(), user); err != nil {
		log.Error().Err(err).Msg("Failed to update user must_change flag")
	}

	log.Info().Str("email", user.Email).Str("temp_password", tempPassword).Msg("Password reset")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":       "password reset successfully",
		"temp_password": tempPassword, // Временное решение
	})
}

// DeleteUser удаляет пользователя (только для админов)
func (u *UsersAPI) DeleteUser(c echo.Context) error {
	// Проверяем права админа
	userRole := c.Get("user_role").(string)
	if userRole != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}

	userID := c.Param("id")
	currentUserID := c.Get("user_id").(string)

	// Нельзя удалить самого себя
	if userID == currentUserID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot delete yourself"})
	}

	// Находим пользователя для проверки email
	user, err := u.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	// Нельзя удалить admin@localhost
	if user.Email == "admin@localhost" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot delete default admin"})
	}

	if err := u.repo.DeleteUser(c.Request().Context(), userID); err != nil {
		log.Error().Err(err).Msg("Failed to delete user")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "user deleted successfully"})
}

// UpdateUser обновляет данные пользователя (только для админов)
func (u *UsersAPI) UpdateUser(c echo.Context) error {
	// Проверяем права админа
	userRole := c.Get("user_role").(string)
	if userRole != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}

	userID := c.Param("id")
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	user, err := u.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	// Нельзя изменить admin@localhost
	if user.Email == "admin@localhost" && req.Email != "admin@localhost" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot modify default admin"})
	}

	user.Email = req.Email
	user.Role = req.Role

	if err := u.repo.UpdateUser(c.Request().Context(), user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "user updated successfully"})
}

// generateTempPassword генерирует временный пароль
func generateTempPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 12)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(1) // Чтобы разные символы
	}
	return string(b)
}
