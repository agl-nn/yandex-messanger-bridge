package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/config"
	"yandex-messenger-bridge/internal/repository/postgres"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/service/webhook"
	"yandex-messenger-bridge/internal/transport/api"
	authMiddleware "yandex-messenger-bridge/internal/transport/middleware"
	"yandex-messenger-bridge/internal/transport/web"
	"yandex-messenger-bridge/internal/yandex"
)

func main() {
	// Добавляем флаг для режима миграции
	migrateOnly := flag.Bool("migrate", false, "Run migrations only and exit")
	flag.Parse()

	// Загружаем конфигурацию
	cfg := config.Load()

	// Подключаемся к БД
	db, err := sqlx.Connect("postgres", cfg.DatabaseDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Выполняем миграции
	log.Info().Msg("Running database migrations...")
	if err := postgres.RunMigrations(db.DB, cfg.DatabaseDSN); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}
	log.Info().Msg("Migrations completed successfully")

	// Если только миграции - завершаем работу
	if *migrateOnly {
		log.Info().Msg("Migration mode: exiting")
		return
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

	// Middleware
	e.Use(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: func(c echo.Context) string {
			if method := c.Request().Header.Get("X-HTTP-Method-Override"); method != "" {
				return method
			}
			return c.FormValue("_method")
		},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:8080"},
		AllowMethods:     []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Публичные webhook эндпоинты
	webhookGroup := e.Group("/webhook")
	webhookGroup.POST("/instance/:id", echo.WrapHandler(http.HandlerFunc(webhookHandler.HandleInstanceWebhook)))

	// Публичные API эндпоинты
	authAPI := api.NewAuthAPI(integrationRepo, cfg.JWTSecret)
	usersAPI := api.NewUsersAPI(integrationRepo, cfg.JWTSecret)

	e.POST("/api/v1/login", authAPI.Login)
	e.POST("/api/v1/logout", authAPI.Logout)

	// Публичные веб-эндпоинты
	webHandler := web.NewHandler(integrationRepo, encryptor)
	e.GET("/login", webHandler.LoginPage)
	e.GET("/change-password", webHandler.ChangePasswordPage)

	// Защищенные API эндпоинты
	authMw := authMiddleware.NewAuthMiddleware(cfg.JWTSecret)
	apiGroup := e.Group("/api/v1")
	apiGroup.Use(authMw.RequireAuth)
	{
		apiGroup.GET("/me", authAPI.Me)
		apiGroup.POST("/change-password", authAPI.ChangePassword)

		// Админские API для управления пользователями
		adminGroup := apiGroup.Group("/admin")
		adminGroup.Use(authMw.RequireAdmin)
		{
			adminGroup.GET("/users", usersAPI.ListUsers)
			adminGroup.POST("/users", usersAPI.CreateUser)
			adminGroup.PUT("/users/:id", usersAPI.UpdateUser)
			adminGroup.DELETE("/users/:id", usersAPI.DeleteUser)
			adminGroup.POST("/users/:id/reset-password", usersAPI.ResetPassword)
		}
	}

	// API для смены пароля (доступно по временному токену)
	tempGroup := e.Group("/api/v1")
	tempGroup.Use(authMw.RequireTempAuth)
	{
		tempGroup.POST("/change-password", authAPI.ChangePassword)
	}

	// Защищенные веб-эндпоинты
	webGroup := e.Group("")
	webGroup.Use(authMw.CookieAuth)
	{
		webGroup.GET("/", webHandler.Dashboard)
		webGroup.GET("/change-password", webHandler.ChangePasswordPage)

		// Админка для пользователей
		adminWebGroup := webGroup.Group("/admin")
		adminWebGroup.Use(authMw.RequireAdmin)
		{
			adminWebGroup.GET("/users", webHandler.UsersAdminPage)
		}

		// Админка для шаблонов
		webGroup.GET("/admin/templates", webHandler.TemplatesAdminPage)
		webGroup.GET("/admin/templates/new", webHandler.TemplateEditPage)
		webGroup.GET("/admin/templates/:id/edit", webHandler.TemplateEditPage)
		webGroup.POST("/admin/templates", webHandler.CreateTemplate)
		webGroup.DELETE("/admin/templates/:id", webHandler.DeleteTemplate)

		// Пользовательские маршруты для шаблонов и экземпляров
		webGroup.GET("/templates", webHandler.TemplatesUserPage)
		webGroup.GET("/templates/custom/new", webHandler.CustomInstanceCreatePage)
		webGroup.POST("/templates/custom", webHandler.CreateCustomInstance)
		webGroup.POST("/instances/custom", webHandler.CreateCustomInstance)
		webGroup.GET("/templates/:id/use", webHandler.InstanceCreatePage)
		webGroup.POST("/instances", webHandler.CreateInstance)
		webGroup.GET("/instances", webHandler.InstancesListPage)
		webGroup.POST("/instances/:id/test", webHandler.TestInstance)
		webGroup.DELETE("/instances/:id", webHandler.DeleteInstance)
		webGroup.GET("/instances/:id/edit", webHandler.EditInstanceForm)
		webGroup.PUT("/instances/:id", webHandler.UpdateInstance)
		webGroup.GET("/instances/:id/last-webhook", webHandler.GetLastWebhook)
	}

	// Статические файлы (иконки уже в образе)
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
		log.Fatal().Err(err).Msg("Failed to shutdown server")
	}
}