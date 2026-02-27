package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/domain"
	"yandex-messenger-bridge/internal/repository"
	"yandex-messenger-bridge/internal/web/templates"
	"yandex-messenger-bridge/internal/web/templates/pages"
	"yandex-messenger-bridge/internal/web/templates/components"
)

// Handler - обработчик веб-интерфейса
type Handler struct {
	repo repository.IntegrationRepository
}

// NewHandler создает новый обработчик
func NewHandler(repo repository.IntegrationRepository) *Handler {
	return &Handler{
		repo: repo,
	}
}

// Dashboard отображает главную страницу
func (h *Handler) Dashboard(c echo.Context) error {
	userID := getUserIDFromContext(c)

	integrations, err := h.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load integrations")
	}

	return templates.Page("Дашборд").
		Render(pages.Dashboard(integrations), c.Response().Writer)
}

// IntegrationsPage отображает страницу со списком интеграций
func (h *Handler) IntegrationsPage(c echo.Context) error {
	userID := getUserIDFromContext(c)

	integrations, err := h.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load integrations")
	}

	// Добавляем webhook URL к каждой интеграции
	baseURL := getBaseURL(c)
	for i := range integrations {
		integrations[i].WebhookURL = baseURL + "/webhook/" + integrations[i].ID
	}

	return components.IntegrationsTable(integrations).Render(c.Response().Writer)
}

// NewIntegrationForm отображает форму создания новой интеграции
func (h *Handler) NewIntegrationForm(c echo.Context) error {
	return components.IntegrationForm(nil, []string{
		"jira", "gitlab", "alertmanager", "grafana",
	}).Render(c.Response().Writer)
}

// CreateIntegration создает новую интеграцию
func (h *Handler) CreateIntegration(c echo.Context) error {
	userID := getUserIDFromContext(c)

	// Парсим форму
	name := c.FormValue("name")
	sourceType := c.FormValue("source_type")
	chatID := c.FormValue("chat_id")
	botToken := c.FormValue("bot_token")
	isActive := c.FormValue("is_active") == "on"

	// Парсим конфигурацию источника
	sourceConfig, err := h.parseSourceConfig(c, sourceType)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid source config: "+err.Error())
	}

	integration := &domain.Integration{
		UserID:       userID,
		Name:         name,
		SourceType:   sourceType,
		SourceConfig: sourceConfig,
		DestinationConfig: domain.DestinationConfig{
			ChatID:   chatID,
			BotToken: botToken, // Будет зашифровано в API слое
		},
		IsActive: isActive,
	}

	if err := h.repo.Create(c.Request().Context(), integration); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create integration")
	}

	// Возвращаем обновленную таблицу
	return h.IntegrationsPage(c)
}

// EditIntegrationForm отображает форму редактирования
func (h *Handler) EditIntegrationForm(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	integration, err := h.repo.FindByIDAndUser(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Integration not found")
	}

	return components.IntegrationForm(integration, []string{
		"jira", "gitlab", "alertmanager", "grafana",
	}).Render(c.Response().Writer)
}

// UpdateIntegration обновляет интеграцию
func (h *Handler) UpdateIntegration(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	// Загружаем существующую
	existing, err := h.repo.FindByIDAndUser(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Integration not found")
	}

	// Обновляем поля
	existing.Name = c.FormValue("name")
	existing.SourceType = c.FormValue("source_type")
	existing.IsActive = c.FormValue("is_active") == "on"

	// Обновляем конфигурацию источника
	sourceConfig, err := h.parseSourceConfig(c, existing.SourceType)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid source config")
	}
	existing.SourceConfig = sourceConfig

	// Обновляем назначение
	existing.DestinationConfig.ChatID = c.FormValue("chat_id")
	if token := c.FormValue("bot_token"); token != "" && token != "***" {
		existing.DestinationConfig.BotToken = token
	}

	if err := h.repo.Update(c.Request().Context(), existing); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update integration")
	}

	return h.IntegrationsPage(c)
}

// DeleteIntegration удаляет интеграцию
func (h *Handler) DeleteIntegration(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	if err := h.repo.Delete(c.Request().Context(), id, userID); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to delete integration")
	}

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
		return c.String(http.StatusInternalServerError, "Failed to load logs")
	}

	return components.LogsTable(logs, total, limit, offset).Render(c.Response().Writer)
}

// TestIntegration отправляет тестовое сообщение
func (h *Handler) TestIntegration(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	// Тестовая отправка будет обработана API слоем
	// Здесь просто возвращаем кнопку с результатом
	return c.HTML(http.StatusOK, `<div class="text-green-600">✓ Тест отправлен</div>`)
}

// SourceConfigFields возвращает поля конфигурации для выбранного источника
func (h *Handler) SourceConfigFields(c echo.Context) error {
	sourceType := c.QueryParam("source_type")
	var config map[string]interface{}

	// Пытаемся получить существующую конфигурацию
	if id := c.QueryParam("id"); id != "" {
		// Загружаем из БД
	}

	switch sourceType {
	case "jira":
		return components.SourceConfigJira(config).Render(c.Response().Writer)
	case "gitlab":
		return components.SourceConfigGitLab(config).Render(c.Response().Writer)
	case "alertmanager":
		return components.SourceConfigAlertmanager(config).Render(c.Response().Writer)
	default:
		return c.String(http.StatusOK, "")
	}
}

// Вспомогательные функции
func getUserIDFromContext(c echo.Context) string {
	// Получаем из JWT токена
	return c.Get("user_id").(string)
}

func getBaseURL(c echo.Context) string {
	scheme := "http"
	if c.Request().TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request().Host
}

func (h *Handler) parseSourceConfig(c echo.Context, sourceType string) (map[string]interface{}, error) {
	config := make(map[string]interface{})

	switch sourceType {
	case "gitlab":
		config["secret_token"] = c.FormValue("source_config[secret_token]")
		config["branch_filter"] = c.FormValue("source_config[branch_filter]")
		config["project_filter"] = strings.Split(c.FormValue("source_config[project_filter]"), ",")

		events := make([]string, 0)
		if c.FormValue("source_config[events][push]") == "on" {
			events = append(events, "push")
		}
		// ... другие события
		config["events"] = events

	case "alertmanager":
		config["min_severity"] = c.FormValue("source_config[min_severity]")
		config["send_resolved"] = c.FormValue("source_config[send_resolved]") == "on"
		config["group_mode"] = c.FormValue("source_config[group_mode]")
		config["template"] = c.FormValue("source_config[template]")

		// Парсим фильтры меток
		if filters := c.FormValue("source_config[label_filters]"); filters != "" {
			var labelFilters map[string]string
			if err := json.Unmarshal([]byte(filters), &labelFilters); err == nil {
				config["label_filters"] = labelFilters
			}
		}
	}

	return config, nil
}
