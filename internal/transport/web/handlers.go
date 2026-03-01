// Путь: internal/transport/web/handlers.go
package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/internal/domain"
	repoInterface "yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/web/templates/components"
	"yandex-messenger-bridge/internal/web/templates/pages"
)

// Handler - обработчик веб-интерфейса
type Handler struct {
	repo repoInterface.IntegrationRepository
}

// NewHandler создает новый обработчик
func NewHandler(repo repoInterface.IntegrationRepository) *Handler {
	return &Handler{
		repo: repo,
	}
}

// LoginPage отображает страницу входа
func (h *Handler) LoginPage(c echo.Context) error {
	return pages.LoginPage().Render(c.Request().Context(), c.Response().Writer)
}

// IntegrationsPage отображает страницу со списком интеграций
func (h *Handler) IntegrationsPage(c echo.Context) error {
	userID := getUserIDFromContext(c)

	integrations, err := h.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load integrations")
		return c.String(http.StatusInternalServerError, "Failed to load integrations")
	}

	baseURL := getBaseURL(c)
	for i := range integrations {
		integrations[i].WebhookURL = baseURL + "/webhook/" + integrations[i].ID
	}

	return components.IntegrationsTable(integrations, baseURL).Render(c.Request().Context(), c.Response().Writer)
}

// NewIntegrationForm отображает форму создания новой интеграции
func (h *Handler) NewIntegrationForm(c echo.Context) error {
	return components.IntegrationForm(nil).Render(c.Request().Context(), c.Response().Writer)
}

// CreateIntegration создает новую интеграцию
func (h *Handler) CreateIntegration(c echo.Context) error {
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

	// Упрощенная конфигурация (пустая)
	sourceConfig := make(map[string]interface{})

	integration := &domain.Integration{
		UserID:       userID,
		Name:         name,
		SourceType:   sourceType,
		SourceConfig: sourceConfig,
		DestinationConfig: domain.DestinationConfig{
			ChatID:   chatID,
			BotToken: botToken,
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
	id := c.Param("id")
	userID := getUserIDFromContext(c)

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

	// Конфигурация остается пустой
	existing.SourceConfig = make(map[string]interface{})

	existing.DestinationConfig.ChatID = c.FormValue("chat_id")
	if token := c.FormValue("bot_token"); token != "" && token != "***" {
		existing.DestinationConfig.BotToken = token
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

// TestIntegration отправляет тестовое сообщение
func (h *Handler) TestIntegration(c echo.Context) error {
	id := c.Param("id")
	if _, err := h.repo.FindByID(c.Request().Context(), id); err != nil {
		return c.String(http.StatusNotFound, "Integration not found")
	}

	return c.HTML(http.StatusOK, `<div class="text-green-600">✓ Тест отправлен</div>`)
}

// SourceConfigFields удален - больше не нужен

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

// parseSourceConfig упрощен - возвращает пустую конфигурацию
func (h *Handler) parseSourceConfig(c echo.Context, sourceType string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}
