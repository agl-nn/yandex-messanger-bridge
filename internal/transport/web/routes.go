package web

import (
	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/transport/middleware"
)

// SetupRoutes настраивает маршруты веб-интерфейса
func SetupRoutes(
	e *echo.Group,
	repo _interface.IntegrationRepository,
	authMiddleware *middleware.AuthMiddleware,
) {
	// Все веб-маршруты требуют аутентификации
	e.Use(authMiddleware.RequireAuth)

	handler := NewHandler(repo)

	// Страницы
	e.GET("", handler.Dashboard)
	e.GET("/", handler.Dashboard)
	e.GET("/integrations", handler.IntegrationsPage)
	e.GET("/integrations/:id/logs", handler.IntegrationLogs)

	// HTMX эндпоинты
	e.GET("/integrations/new", handler.NewIntegrationForm)
	e.POST("/integrations", handler.CreateIntegration)
	e.GET("/integrations/:id/edit", handler.EditIntegrationForm)
	e.PUT("/integrations/:id", handler.UpdateIntegration)
	e.DELETE("/integrations/:id", handler.DeleteIntegration)
	e.POST("/integrations/:id/test", handler.TestIntegration)

	// Динамические формы
	e.GET("/integrations/source-config-fields", handler.SourceConfigFields)
}
