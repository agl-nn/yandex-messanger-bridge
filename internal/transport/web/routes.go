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
		protected.POST("/logout", handler.Logout)

		// Админка для шаблонов
		protected.GET("/admin/templates", handler.TemplatesAdminPage)
		protected.GET("/admin/templates/new", handler.TemplateEditPage)
		protected.GET("/admin/templates/:id/edit", handler.TemplateEditPage)
		protected.POST("/admin/templates", handler.CreateTemplate)
		protected.DELETE("/admin/templates/:id", handler.DeleteTemplate)

		// Пользовательские маршруты для шаблонов и экземпляров
		protected.GET("/templates", handler.TemplatesUserPage)
		protected.GET("/templates/:id/use", handler.InstanceCreatePage)
		protected.POST("/instances", handler.CreateInstance)
		protected.GET("/instances", handler.InstancesListPage)
		protected.POST("/instances/:id/test", handler.TestInstance)
		protected.DELETE("/instances/:id", handler.DeleteInstance)
		protected.GET("/instances/:id/edit", handler.EditInstanceForm)
		protected.PUT("/instances/:id", handler.UpdateInstance)
		protected.GET("/instances/:id/last-webhook", handler.GetLastWebhook)
	}
}
