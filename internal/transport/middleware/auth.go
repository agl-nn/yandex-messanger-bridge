package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"yandex-messenger-bridge/internal/repository/interface"
)

// AuthMiddleware - middleware для аутентификации
type AuthMiddleware struct {
	repo      _interface.IntegrationRepository
	jwtSecret []byte
}

// NewAuthMiddleware создает новый middleware
func NewAuthMiddleware(repo _interface.IntegrationRepository, jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
	}
}

// RequireAuth требует аутентификации через JWT
func (m *AuthMiddleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := extractToken(c.Request())
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token"})
		}

		claims, err := m.validateJWT(token)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		}

		// Сохраняем user_id в контексте
		c.Set("user_id", claims["user_id"])
		c.Set("user_role", claims["role"])

		return next(c)
	}
}

// RequireAPIKey требует аутентификации через API ключ
func (m *AuthMiddleware) RequireAPIKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey := c.Request().Header.Get("X-API-Key")
		if apiKey == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing API key"})
		}

		// Хешируем ключ для поиска в БД
		hash := hashAPIKey(apiKey)

		key, err := m.repo.FindAPIKeyByHash(c.Request().Context(), hash)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		}

		// Проверяем срок действия
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "API key expired"})
		}

		// Обновляем время последнего использования
		go m.repo.UpdateAPIKeyLastUsed(c.Request().Context(), key.ID)

		c.Set("user_id", key.UserID)

		return next(c)
	}
}

// GenerateJWT генерирует JWT токен для пользователя
func (m *AuthMiddleware) GenerateJWT(userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.jwtSecret)
}

// validateJWT проверяет JWT токен
func (m *AuthMiddleware) validateJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, echo.ErrUnauthorized
		}
		return m.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, echo.ErrUnauthorized
}

// HashPassword хеширует пароль
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword проверяет пароль
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// hashAPIKey хеширует API ключ (простое хеширование, не для обратной расшифровки)
func hashAPIKey(key string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	return string(hash)
}

// Вспомогательная функция для извлечения токена из заголовка
func extractToken(r *http.Request) string {
	bearToken := r.Header.Get("Authorization")
	parts := strings.Split(bearToken, " ")
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}
	return ""
}
