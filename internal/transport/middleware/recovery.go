package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RecoveryMiddleware восстанавливается после паник
func RecoveryMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				// Логируем стек вызовов
				log.Error().
					Interface("panic", r).
					Str("stack", string(debug.Stack())).
					Str("path", c.Request().URL.Path).
					Msg("panic recovered")

				// Отправляем 500 ошибку
				c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()

		return next(c)
	}
}

// Recovery возвращает middleware для восстановления
func Recovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return RecoveryMiddleware(next)
	}
}
