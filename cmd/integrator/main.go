package main

import (
	"context"
	"log"
	"net/http"
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
	authMiddleware "yandex-messenger-bridge/internal/transport/middleware"
	"yandex-messenger-bridge/internal/service/webhook"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/yandex"
)

func main() {
	cfg := config.Load()

	db, err := sqlx.Connect("postgres", cfg.DatabaseDSN)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db.DB, cfg.DatabaseDSN); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	integrationRepo := postgres.NewIntegrationRepository(db)
	encryptor := encryption.NewEncryptor(cfg.EncryptionKey)
	yandexClient := yandex.NewClient("")

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

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Публичные webhook эндпоинты
	webhookGroup := e.Group("/webhook")
	webhookGroup.POST("/:id/jira", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleJira)))
	webhookGroup.POST("/:id/gitlab", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleGitLab)))
	webhookGroup.POST("/:id/alertmanager", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleAlertmanager)))

	// Публичные API эндпоинты (аутентификация)
	authAPI := api.NewAuthAPI(integrationRepo, cfg.JWTSecret)
	e.POST("/api/v1/login", authAPI.Login)

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
		apiGroup.GET("/me", authAPI.Me)
	}

	// Защищенные веб-эндпоинты
	webHandler := web.NewHandler(integrationRepo)
	webGroup := e.Group("")
	webGroup.Use(authMw.RequireAuth)
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
		webGroup.GET("/integrations/source-config-fields", webHandler.SourceConfigFields)
	}

	e.Static("/static", "internal/web/static")

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
