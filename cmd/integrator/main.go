package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/config"
	"yandex-messenger-bridge/internal/repository/postgres"
	"yandex-messenger-bridge/internal/transport/api"
	"yandex-messenger-bridge/internal/transport/web"
	authMiddleware "yandex-messenger-bridge/internal/transport/middleware"
	"yandex-messenger-bridge/internal/service/webhook"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/yandex"
)

func main() {
	// Загружаем конфигурацию
	cfg := config.Load()

	// Подключаемся к БД
	db, err := sqlx.Connect("postgres", cfg.DatabaseDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Выполняем миграции
	if err := postgres.RunMigrations(db.DB, cfg.DatabaseDSN); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Инициализируем сервисы
	encryptor := encryption.NewEncryptor(cfg.EncryptionKey)
	yandexClient := yandex.NewClient("") // Токен будет подставляться динамически

	// Инициализируем репозитории
	integrationRepo := postgres.NewIntegrationRepository(db, encryptor)

	// Инициализируем обработчики вебхуков
	webhookHandler := webhook.NewHandler(
		integrationRepo,
		yandexClient,
		encryptor,
		webhook.Config{
			GitLabTimeout:       10 * time.Second,
			AlertmanagerTimeout: 5 * time.Second,
			JiraTimeout:         10 * time.Second,
			MaxRetries:          3,
		},
	)

	// Создаем Echo сервер
	e := echo.New()

	// Middleware (ВАЖЕН ПОРЯДОК!)
	e.Use(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: func(c echo.Context) string {
			// Сначала проверяем заголовок от HTMX
			if method := c.Request().Header.Get("X-HTTP-Method-Override"); method != "" {
				return method
			}
			// Затем проверяем поле формы (для _method)
			return c.FormValue("_method")
		},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Публичные webhook эндпоинты (без аутентификации)
	webhookGroup := e.Group("/webhook")
	webhookGroup.POST("/:id/jira", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleJira)))
	webhookGroup.POST("/:id/gitlab", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleGitLab)))
	webhookGroup.POST("/:id/alertmanager", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleAlertmanager)))

	// Публичные API эндпоинты
	authAPI := api.NewAuthAPI(integrationRepo, cfg.JWTSecret)
	e.POST("/api/v1/login", authAPI.Login)

	// Публичные веб-эндпоинты
	webHandler := web.NewHandler(integrationRepo, encryptor)
	e.GET("/login", webHandler.LoginPage)

	// Защищенные API эндпоинты
	authMw := authMiddleware.NewAuthMiddleware(cfg.JWTSecret)
	apiGroup := e.Group("/api/v1")
	apiGroup.Use(authMw.RequireAuth)
	{
		integrationAPI := api.NewIntegrationAPI(integrationRepo, encryptor, cfg.BaseURL)
		apiGroup.GET("/integrations", integrationAPI.List)
		apiGroup.POST("/integrations", integrationAPI.Create)
		apiGroup.GET("/integrations/:id", integrationAPI.Get)
		apiGroup.PUT("/integrations/:id", integrationAPI.Update)
		apiGroup.DELETE("/integrations/:id", integrationAPI.Delete)
		apiGroup.GET("/integrations/:id/logs", integrationAPI.GetLogs)
		apiGroup.POST("/integrations/:id/test", integrationAPI.Test)
		apiGroup.POST("/integrations/jira", integrationAPI.CreateJira)
		apiGroup.POST("/integrations/gitlab", integrationAPI.CreateGitLab)
		apiGroup.POST("/integrations/custom", integrationAPI.CreateCustom)
		apiGroup.POST("/integrations/custom", integrationAPI.CreateCustom)
		apiGroup.GET("/me", authAPI.Me)
	}
	// ТЕСТОВЫЙ ОБРАБОТЧИК - ВРЕМЕННО
	e.PUT("/integrations/:id", func(c echo.Context) error {
		log.Info().
			Str("id", c.Param("id")).
			Str("handler", "DIRECT_ECHO").
			Msg("🔥🔥🔥 DIRECT ECHO HANDLER CALLED 🔥🔥🔥")
		return c.String(http.StatusOK, "Direct echo handler OK")
	})
	// Защищенные веб-эндпоинты (с токеном в cookie)
	webGroup := e.Group("")
	webGroup.Use(authMw.CookieAuth)
	{
		webGroup.GET("/", webHandler.Dashboard)
		webGroup.GET("/integrations", webHandler.IntegrationsPage)
		webGroup.GET("/integrations/new", webHandler.NewIntegrationForm)
		webGroup.POST("/integrations", webHandler.CreateIntegration)
		webGroup.GET("/integrations/:id/edit", webHandler.EditIntegrationForm)
		webGroup.PUT("/integrations/:id", webHandler.UpdateIntegration)
		webGroup.DELETE("/integrations/:id", webHandler.DeleteIntegration)
		webGroup.GET("/integrations/:id/logs", webHandler.IntegrationLogs)
		webGroup.POST("/integrations/:id/test", webHandler.TestIntegration)
		webGroup.POST("/logout", webHandler.Logout)
	}

	// Статические файлы
	e.Static("/static", "internal/web/static")

	// Отладка: показать все зарегистрированные маршруты
	for _, route := range e.Routes() {
		log.Info().Str("method", route.Method).Str("path", route.Path).Msg("Registered route")
	}

	// Graceful shutdown
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to shutdown server")
	}
}
