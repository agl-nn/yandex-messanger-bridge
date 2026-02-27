package api

import (
	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/transport/middleware"
)

// SetupRoutes настраивает маршруты API
func SetupRoutes(
	e *echo.Group,
	repo _interface.IntegrationRepository,
	encryptor *encryption.Encryptor,
	authMiddleware *middleware.AuthMiddleware,
	baseURL string,
) {
	// Публичные маршруты (без аутентификации)
	authAPI := NewAuthAPI(repo, authMiddleware)
	e.POST("/auth/login", authAPI.Login)
	e.POST("/auth/register", authAPI.Register)

	// Защищенные маршруты (требуют JWT)
	protected := e.Group("")
	protected.Use(authMiddleware.RequireAuth)

	// Интеграции
	integrationAPI := NewIntegrationAPI(repo, encryptor, baseURL)
	protected.GET("/integrations", integrationAPI.List)
	protected.POST("/integrations", integrationAPI.Create)
	protected.GET("/integrations/:id", integrationAPI.Get)
	protected.PUT("/integrations/:id", integrationAPI.Update)
	protected.DELETE("/integrations/:id", integrationAPI.Delete)
	protected.GET("/integrations/:id/logs", integrationAPI.GetLogs)
	protected.POST("/integrations/:id/test", integrationAPI.Test)

	// Пользователь
	protected.GET("/me", authAPI.Me)

	// API ключи (TODO)
	// protected.GET("/api-keys", ...)

	// Маршруты для API ключей (альтернативная аутентификация)
	apiKeyGroup := e.Group("")
	apiKeyGroup.Use(authMiddleware.RequireAPIKey)
	// Дублируем основные маршруты для API ключей
	apiKeyGroup.GET("/integrations", integrationAPI.List)
	apiKeyGroup.GET("/integrations/:id", integrationAPI.Get)
}
