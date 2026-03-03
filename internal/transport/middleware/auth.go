package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type AuthMiddleware struct {
	jwtSecret []byte
}

func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: []byte(jwtSecret),
	}
}

// RequireAuth проверяет токен в заголовке (для API)
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

		c.Set("user_id", claims["user_id"])
		c.Set("user_role", claims["role"])

		return next(c)
	}
}

// CookieAuth проверяет токен в cookie (для веб-интерфейса) с редиректом для браузеров
func (m *AuthMiddleware) CookieAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Проверка токена в заголовке (для API)
		token := extractToken(c.Request())
		if token != "" {
			claims, err := m.validateJWT(token)
			if err == nil {
				c.Set("user_id", claims["user_id"])
				c.Set("user_role", claims["role"])
				return next(c)
			}
		}

		// Проверка токена в cookie
		cookie, err := c.Cookie("token")
		if err == nil {
			claims, err := m.validateJWT(cookie.Value)
			if err == nil {
				c.Set("user_id", claims["user_id"])
				c.Set("user_role", claims["role"])
				return next(c)
			}
		}

		// Если нет токена, определяем тип клиента по заголовку Accept
		accept := c.Request().Header.Get("Accept")
		if strings.Contains(accept, "text/html") {
			// Для браузера - редирект на страницу логина
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		// Для API-клиентов возвращаем JSON
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token"})
	}
}

func extractToken(r *http.Request) string {
	bearToken := r.Header.Get("Authorization")
	parts := strings.Split(bearToken, " ")
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}
	return ""
}

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
