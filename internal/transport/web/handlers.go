// Путь: internal/transport/web/handlers.go
package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/internal/domain"
	repoInterface "yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/yandex"
	"yandex-messenger-bridge/internal/web/templates/components"
	"yandex-messenger-bridge/internal/web/templates/pages"
)

// Handler - обработчик веб-интерфейса
type Handler struct {
	repo      repoInterface.IntegrationRepository
	encryptor *encryption.Encryptor
}

// NewHandler создает новый обработчик
func NewHandler(repo repoInterface.IntegrationRepository, encryptor *encryption.Encryptor) *Handler {
	return &Handler{
		repo:      repo,
		encryptor: encryptor,
	}
}

// LoginPage отображает страницу входа
func (h *Handler) LoginPage(c echo.Context) error {
	return pages.LoginPage().Render(c.Request().Context(), c.Response().Writer)
}

// IntegrationsPage отображает страницу со списком интеграций
func (h *Handler) IntegrationsPage(c echo.Context) error {
	userID := getUserIDFromContext(c)

	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
	}

	integrations, err := h.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load integrations")
		return c.String(http.StatusInternalServerError, "Failed to load integrations")
	}

	baseURL := getBaseURL(c)
	for i := range integrations {
		integrations[i].WebhookURL = baseURL + "/webhook/" + integrations[i].ID
	}

	return pages.IntegrationsPage(integrations, baseURL, user).Render(c.Request().Context(), c.Response().Writer)
}

// NewIntegrationForm отображает форму создания новой интеграции
func (h *Handler) NewIntegrationForm(c echo.Context) error {
	return components.IntegrationForm(nil).Render(c.Request().Context(), c.Response().Writer)
}

// CreateIntegration создает новую интеграцию
func (h *Handler) CreateIntegration(c echo.Context) error {
	log.Info().
		Str("_method", c.FormValue("_method")).
		Msg("🟢 CREATE INTEGRATION CALLED")

	userID := getUserIDFromContext(c)
	log.Info().Str("user_id", userID).Msg("Creating integration")

	name := c.FormValue("name")
	sourceType := c.FormValue("source_type")
	chatID := c.FormValue("chat_id")
	botToken := c.FormValue("bot_token")
	isActive := c.FormValue("is_active") == "on"

	log.Info().
		Str("name", name).
		Str("source_type", sourceType).
		Str("chat_id", chatID).
		Bool("is_active", isActive).
		Msg("Form values")

	// Шифруем токен перед сохранением
	encryptedToken, err := h.encryptor.Encrypt(botToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encrypt bot token")
		return c.String(http.StatusInternalServerError, "Failed to encrypt token")
	}

	sourceConfig := make(map[string]interface{})

	integration := &domain.Integration{
		UserID:       userID,
		Name:         name,
		SourceType:   sourceType,
		SourceConfig: sourceConfig,
		DestinationConfig: domain.DestinationConfig{
			ChatID:   chatID,
			BotToken: encryptedToken,
		},
		IsActive: isActive,
	}

	if err := h.repo.Create(c.Request().Context(), integration); err != nil {
		log.Error().Err(err).Msg("Failed to create integration")
		return c.String(http.StatusInternalServerError, "Failed to create integration")
	}

	log.Info().Str("id", integration.ID).Msg("Integration created successfully")
	return h.IntegrationsPage(c)
}

// EditIntegrationForm отображает форму редактирования
func (h *Handler) EditIntegrationForm(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	integration, err := h.repo.FindByID(c.Request().Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Integration not found")
		return c.String(http.StatusNotFound, "Integration not found")
	}

	if integration.UserID != userID {
		log.Warn().Str("user_id", userID).Str("owner_id", integration.UserID).Msg("Access denied")
		return c.String(http.StatusForbidden, "Access denied")
	}

	return components.IntegrationForm(integration).Render(c.Request().Context(), c.Response().Writer)
}

// UpdateIntegration обновляет интеграцию
func (h *Handler) UpdateIntegration(c echo.Context) error {
	log.Info().Msg("🚀🚀🚀 UpdateIntegration HANDLER CALLED 🚀🚀🚀")
	// Логируем ВСЕ параметры запроса
	log.Info().
		Str("method", c.Request().Method).
		Str("_method", c.FormValue("_method")).
		Str("id_from_param", c.Param("id")).
		Str("id_from_form", c.FormValue("id")).
		Str("name", c.FormValue("name")).
		Str("source_type", c.FormValue("source_type")).
		Str("chat_id", c.FormValue("chat_id")).
		Str("is_active", c.FormValue("is_active")).
		Msg("🔵 UPDATE INTEGRATION CALLED - RAW REQUEST")

	id := c.Param("id")
	userID := getUserIDFromContext(c)

	// Логируем уже обработанные данные
	log.Info().
		Str("method", c.Request().Method).
		Str("_method", c.FormValue("_method")).
		Str("id", id).
		Str("name", c.FormValue("name")).
		Str("source_type", c.FormValue("source_type")).
		Bool("is_active", c.FormValue("is_active") == "on").
		Msg("Update request received")

	existing, err := h.repo.FindByID(c.Request().Context(), id)
	if err != nil {
		return c.String(http.StatusNotFound, "Integration not found")
	}

	if existing.UserID != userID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	existing.Name = c.FormValue("name")
	existing.SourceType = c.FormValue("source_type")
	existing.IsActive = c.FormValue("is_active") == "on"
	existing.DestinationConfig.ChatID = c.FormValue("chat_id")

	if token := c.FormValue("bot_token"); token != "" && token != "***" {
		encryptedToken, err := h.encryptor.Encrypt(token)
		if err != nil {
			log.Error().Err(err).Msg("Failed to encrypt bot token")
			return c.String(http.StatusInternalServerError, "Failed to encrypt token")
		}
		existing.DestinationConfig.BotToken = encryptedToken
	}

	if err := h.repo.Update(c.Request().Context(), existing); err != nil {
		log.Error().Err(err).Msg("Failed to update integration")
		return c.String(http.StatusInternalServerError, "Failed to update integration")
	}

	log.Info().Str("id", id).Msg("Integration updated successfully")
	return h.IntegrationsPage(c)
}

// DeleteIntegration удаляет интеграцию
func (h *Handler) DeleteIntegration(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	if err := h.repo.Delete(c.Request().Context(), id, userID); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete integration")
		return c.String(http.StatusInternalServerError, "Failed to delete integration")
	}

	log.Info().Str("id", id).Msg("Integration deleted successfully")
	return h.IntegrationsPage(c)
}

// IntegrationLogs отображает логи доставки для интеграции
func (h *Handler) IntegrationLogs(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	logs, total, err := h.repo.GetDeliveryLogs(c.Request().Context(), id, userID, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to load logs")
		return c.String(http.StatusInternalServerError, "Failed to load logs")
	}

	return components.LogsTable(logs, total, limit, offset).Render(c.Request().Context(), c.Response().Writer)
}

// TestIntegration отправляет тестовое сообщение в Яндекс.Мессенджер
func (h *Handler) TestIntegration(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	// Загружаем интеграцию
	integration, err := h.repo.FindByIDAndUser(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Интеграция не найдена")
	}

	// Расшифровываем токен бота
	decryptedToken, err := h.encryptor.Decrypt(integration.DestinationConfig.BotToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decrypt bot token")
		return c.HTML(http.StatusInternalServerError, `<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">Ошибка расшифровки токена</div>`)
	}

	// Создаём клиент Яндекс.Мессенджера
	yandexClient := yandex.NewClient(decryptedToken)

	// Формируем тестовое сообщение
	testMessage := fmt.Sprintf("🔄 *Тестовое сообщение*\n\nИнтеграция: *%s*\nТип: *%s*\nВремя: *%s*",
		integration.Name,
		integration.SourceType,
		time.Now().Format("02.01.2006 15:04:05"))

	// Отправляем сообщение
	err = yandexClient.SendToChat(c.Request().Context(), integration.DestinationConfig.ChatID, testMessage, nil)

	if err != nil {
		log.Error().Err(err).Str("integration_id", id).Msg("Test message failed")
		return c.HTML(http.StatusInternalServerError, fmt.Sprintf(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">Ошибка: %s</div>`, err.Error()))
	}

	log.Info().Str("integration_id", id).Msg("Test message sent successfully")
	return c.HTML(http.StatusOK, `<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded">✓ Тестовое сообщение отправлено</div>`)
}

// Logout обрабатывает выход из системы
func (h *Handler) Logout(c echo.Context) error {
	// Удаляем cookie
	c.SetCookie(&http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-24 * time.Hour),
		HttpOnly: true,
	})

	return c.NoContent(http.StatusOK)
}

// Вспомогательные функции
func getUserIDFromContext(c echo.Context) string {
	userID := c.Get("user_id")
	if userID == nil {
		return ""
	}
	return userID.(string)
}

func getBaseURL(c echo.Context) string {
	scheme := "http"
	if c.Request().TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request().Host
}

func (h *Handler) parseSourceConfig(c echo.Context, sourceType string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

// ================ Обработчики для шаблонов (админка) ================

// TemplatesAdminPage отображает страницу управления шаблонами
func (h *Handler) TemplatesAdminPage(c echo.Context) error {
	userID := getUserIDFromContext(c)

	// Проверяем, что пользователь админ
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		log.Error().Err(err).Str("role", user.Role).Msg("Access denied to admin page")
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	// Получаем все шаблоны (для админа показываем все)
	templates, err := h.repo.ListTemplates(c.Request().Context(), userID, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load templates")
		return c.String(http.StatusInternalServerError, "Failed to load templates")
	}

	// ЛОГИРУЕМ КОЛИЧЕСТВО ШАБЛОНОВ
	log.Info().Int("count", len(templates)).Msg("Templates loaded for admin page")
	for _, t := range templates {
		log.Info().Str("id", t.ID).Str("name", t.Name).Str("icon", t.Icon).Msg("Template details")
	}

	return pages.TemplatesAdminPage(templates, user).Render(c.Request().Context(), c.Response().Writer)
}

// NewTemplateForm отображает форму создания нового шаблона
func (h *Handler) NewTemplateForm(c echo.Context) error {
	userID := getUserIDFromContext(c)

	// Проверяем права админа
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	return components.TemplateForm(nil).Render(c.Request().Context(), c.Response().Writer)
}

// CreateTemplate создает новый шаблон
func (h *Handler) CreateTemplate(c echo.Context) error {
	userID := getUserIDFromContext(c)

	// Проверяем права админа
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		log.Error().Err(err).Str("role", user.Role).Msg("Access denied")
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	name := c.FormValue("name")
	icon := c.FormValue("icon")
	description := c.FormValue("description")
	templateText := c.FormValue("template_text")
	isPublic := c.FormValue("is_public") == "on"

	if name == "" || templateText == "" {
		return c.String(http.StatusBadRequest, "Name and template text are required")
	}

	template := &domain.Template{
		Name:         name,
		Icon:         icon,
		Description:  description,
		TemplateText: templateText,
		IsPublic:     isPublic,
		CreatedBy:    sql.NullString{String: userID, Valid: userID != ""},
	}

	if err := h.repo.CreateTemplate(c.Request().Context(), template); err != nil {
		log.Error().Err(err).Msg("Failed to create template")
		return c.String(http.StatusInternalServerError, "Failed to create template")
	}

	log.Info().Str("id", template.ID).Str("name", name).Msg("Template created")

	// ВАЖНО: редирект на страницу со списком шаблонов
	return c.Redirect(http.StatusSeeOther, "/admin/templates")
}

// EditTemplateForm отображает форму редактирования шаблона
func (h *Handler) EditTemplateForm(c echo.Context) error {
	userID := getUserIDFromContext(c)
	id := c.Param("id")

	// Проверяем права админа
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	template, err := h.repo.GetTemplateByID(c.Request().Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Template not found")
		return c.String(http.StatusNotFound, "Template not found")
	}

	return components.TemplateForm(template).Render(c.Request().Context(), c.Response().Writer)
}

// UpdateTemplate обновляет шаблон
func (h *Handler) UpdateTemplate(c echo.Context) error {
	userID := getUserIDFromContext(c)
	id := c.Param("id")

	// Проверяем права админа
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	// Проверяем существование шаблона
	existing, err := h.repo.GetTemplateByID(c.Request().Context(), id)
	if err != nil {
		return c.String(http.StatusNotFound, "Template not found")
	}

	existing.Name = c.FormValue("name")
	existing.Icon = c.FormValue("icon")
	existing.Description = c.FormValue("description")
	existing.TemplateText = c.FormValue("template_text")
	existing.IsPublic = c.FormValue("is_public") == "on"

	if err := h.repo.UpdateTemplate(c.Request().Context(), existing); err != nil {
		log.Error().Err(err).Msg("Failed to update template")
		return c.String(http.StatusInternalServerError, "Failed to update template")
	}

	log.Info().Str("id", id).Msg("Template updated")
	return c.Redirect(http.StatusSeeOther, "/admin/templates")
}

// DeleteTemplate удаляет шаблон
func (h *Handler) DeleteTemplate(c echo.Context) error {
	userID := getUserIDFromContext(c)
	id := c.Param("id")

	// Проверяем права админа
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	if err := h.repo.DeleteTemplate(c.Request().Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete template")
		return c.String(http.StatusInternalServerError, "Failed to delete template")
	}

	log.Info().Str("id", id).Msg("Template deleted")
	return h.TemplatesAdminPage(c)
}

// TemplateEditPage отображает страницу создания/редактирования шаблона
func (h *Handler) TemplateEditPage(c echo.Context) error {
	userID := getUserIDFromContext(c)
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	id := c.Param("id")
	var template *domain.Template
	if id != "" && id != "new" {
		template, err = h.repo.GetTemplateByID(c.Request().Context(), id)
		if err != nil {
			return c.String(http.StatusNotFound, "Template not found")
		}
	}

	return pages.TemplateEditPage(template, user).Render(c.Request().Context(), c.Response().Writer)
}

// CreateTemplate создает новый шаблон
func (h *Handler) CreateTemplate(c echo.Context) error {
	userID := getUserIDFromContext(c)

	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil || user.Role != "admin" {
		return c.String(http.StatusForbidden, "Доступ запрещен")
	}

	name := c.FormValue("name")
	icon := c.FormValue("icon")
	description := c.FormValue("description")
	templateText := c.FormValue("template_text")
	isPublic := c.FormValue("is_public") == "on"
	id := c.FormValue("id")

	if name == "" || templateText == "" {
		return c.String(http.StatusBadRequest, "Name and template text are required")
	}

	if id != "" {
		// Обновление существующего шаблона
		template, err := h.repo.GetTemplateByID(c.Request().Context(), id)
		if err != nil {
			return c.String(http.StatusNotFound, "Template not found")
		}

		template.Name = name
		template.Icon = icon
		template.Description = description
		template.TemplateText = templateText
		template.IsPublic = isPublic

		if err := h.repo.UpdateTemplate(c.Request().Context(), template); err != nil {
			log.Error().Err(err).Msg("Failed to update template")
			return c.String(http.StatusInternalServerError, "Failed to update template")
		}
		log.Info().Str("id", id).Str("name", name).Msg("Template updated")
	} else {
		// Создание нового шаблона
		template := &domain.Template{
			Name:         name,
			Icon:         icon,
			Description:  description,
			TemplateText: templateText,
			IsPublic:     isPublic,
			CreatedBy:    sql.NullString{String: userID, Valid: userID != ""},
		}

		if err := h.repo.CreateTemplate(c.Request().Context(), template); err != nil {
			log.Error().Err(err).Msg("Failed to create template")
			return c.String(http.StatusInternalServerError, "Failed to create template")
		}
		log.Info().Str("id", template.ID).Str("name", name).Msg("Template created")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/templates")
}
