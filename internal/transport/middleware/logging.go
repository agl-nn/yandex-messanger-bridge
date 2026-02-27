package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// LoggerMiddleware логирует все запросы
func LoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		err := next(c)

		stop := time.Now()
		latency := stop.Sub(start)

		logEvent := log.Info()
		if err != nil {
			logEvent = log.Error().Err(err)
		}

		logEvent.Str("method", c.Request().Method).
			Str("path", c.Request().URL.Path).
			Int("status", c.Response().Status).
			Str("ip", c.RealIP()).
			Dur("latency", latency).
			Str("user_agent", c.Request().UserAgent()).
			Msg("request")

		return err
	}
}

// RequestLogger возвращает middleware для логирования
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return LoggerMiddleware(next)
	}
}
