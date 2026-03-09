package web

import (
	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/transport/middleware"
	"yandex-messenger-bridge/internal/service/encryption"
)

func SetupRoutes(
	e *echo.Group,
	repo _interface.IntegrationRepository,
	authMiddleware *middleware.AuthMiddleware,
	encryptor *encryption.Encryptor,
) {
	handler := NewHandler(repo, encryptor)

	// Публичные маршруты
	e.GET("/login", handler.LoginPage)

	// Защищенные маршруты
	protected := e.Group("")
	protected.Use(authMiddleware.CookieAuth)
	{
		protected.GET("/", handler.Dashboard)
		protected.GET("/integrations", handler.IntegrationsPage)
		protected.GET("/integrations/new", handler.NewIntegrationForm)
		protected.POST("/integrations", handler.CreateIntegration)
		protected.GET("/integrations/:id/edit", handler.EditIntegrationForm)
		protected.PUT("/integrations/:id", handler.UpdateIntegration)
		protected.DELETE("/integrations/:id", handler.DeleteIntegration)
		protected.POST("/integrations/:id/test", handler.TestIntegration)
		// Удален вызов handler.SourceConfigFields
	}
}
