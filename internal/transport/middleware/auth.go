package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	//"github.com/rs/zerolog/log"
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

		// Проверяем, не временный ли это токен
		if mustChange, ok := claims["must_change"].(bool); ok && mustChange {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "must change password"})
		}

		c.Set("user_id", claims["user_id"])
		c.Set("user_role", claims["role"])

		return next(c)
	}
}

// RequireTempAuth проверяет временный токен (для смены пароля)
func (m *AuthMiddleware) RequireTempAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Проверяем temp_token в cookie
		cookie, err := c.Cookie("temp_token")
		if err == nil {
			claims, err := m.validateJWT(cookie.Value)
			if err == nil {
				c.Set("user_id", claims["user_id"])
				c.Set("user_role", claims["role"])
				return next(c)
			}
		}

		// Если нет temp_token, пробуем обычный токен (для уже авторизованных)
		token := extractToken(c.Request())
		if token != "" {
			claims, err := m.validateJWT(token)
			if err == nil {
				c.Set("user_id", claims["user_id"])
				c.Set("user_role", claims["role"])
				return next(c)
			}
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
}

// CookieAuth проверяет токен в cookie (для веб-интерфейса)
func (m *AuthMiddleware) CookieAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
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

		// Проверка временного токена (для страницы смены пароля)
		tempCookie, err := c.Cookie("temp_token")
		if err == nil {
			claims, err := m.validateJWT(tempCookie.Value)
			if err == nil {
				// Если запрос на /change-password, пропускаем
				if c.Path() == "/change-password" {
					c.Set("user_id", claims["user_id"])
					c.Set("user_role", claims["role"])
					return next(c)
				}
				// Иначе редирект на смену пароля
				return c.Redirect(http.StatusSeeOther, "/change-password")
			}
		}

		// Если нет токена, редирект на логин
		accept := c.Request().Header.Get("Accept")
		if strings.Contains(accept, "text/html") {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token"})
	}
}

// RequireAdmin проверяет права администратора
func (m *AuthMiddleware) RequireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		role := c.Get("user_role")
		if role == nil || role.(string) != "admin" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "admin access required"})
		}
		return next(c)
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
