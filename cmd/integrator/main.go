package main

import (
	"context"
	"log"
	"net/http" // ЭТОТ ИМПОРТ НУЖЕН!
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"yandex-messenger-bridge/config"
	"yandex-messenger-bridge/internal/repository/postgres"
	"yandex-messenger-bridge/internal/transport/api"
	"yandex-messenger-bridge/internal/transport/web"
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
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Выполняем миграции
	if err := postgres.RunMigrations(db.DB, cfg.DatabaseDSN); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Инициализируем репозитории
	integrationRepo := postgres.NewIntegrationRepository(db)

	// Инициализируем сервисы
	encryptor := encryption.NewEncryptor(cfg.EncryptionKey)
	yandexClient := yandex.NewClient("") // Токен будет подставляться динамически

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

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Публичные webhook эндпоинты (без аутентификации)
	webhookGroup := e.Group("/webhook")
	webhookGroup.POST("/:id/jira", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleJira)))
	webhookGroup.POST("/:id/gitlab", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleGitLab)))
	webhookGroup.POST("/:id/alertmanager", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleAlertmanager)))

	// API для фронтенда (с аутентификацией)
	apiGroup := e.Group("/api/v1")
	{
		integrationAPI := api.NewIntegrationAPI(integrationRepo, encryptor, cfg.BaseURL)
		apiGroup.GET("/integrations", integrationAPI.List)
		apiGroup.POST("/integrations", integrationAPI.Create)
		apiGroup.GET("/integrations/:id", integrationAPI.Get)
		apiGroup.PUT("/integrations/:id", integrationAPI.Update)
		apiGroup.DELETE("/integrations/:id", integrationAPI.Delete)
		apiGroup.GET("/integrations/:id/logs", integrationAPI.GetLogs)
		apiGroup.POST("/integrations/:id/test", integrationAPI.Test)
	}

	// Веб-интерфейс (пока без аутентификации для теста)
	webHandler := web.NewHandler(integrationRepo)
	e.GET("/", webHandler.Dashboard)
	e.GET("/integrations", webHandler.IntegrationsPage)
	e.GET("/integrations/new", webHandler.NewIntegrationForm)
	e.POST("/integrations", webHandler.CreateIntegration)
	e.GET("/integrations/:id/edit", webHandler.EditIntegrationForm)
	e.PUT("/integrations/:id", webHandler.UpdateIntegration)
	e.DELETE("/integrations/:id", webHandler.DeleteIntegration)
	e.GET("/integrations/:id/logs", webHandler.IntegrationLogs)
	e.POST("/integrations/:id/test", webHandler.TestIntegration)
	e.GET("/integrations/source-config-fields", webHandler.SourceConfigFields)

	// Статические файлы
	e.Static("/static", "internal/web/static")

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
		log.Fatal(err)
	}
}
