// Путь: internal/transport/web/handlers.go
package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
	"bytes"
	"encoding/json"
	//"math"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/internal/domain"
	repoInterface "yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/yandex"
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

// Dashboard отображает главную страницу с дашбордом
func (h *Handler) Dashboard(c echo.Context) error {
	userID := getUserIDFromContext(c)
	log.Info().Str("user_id", userID).Msg("Dashboard accessed")

	if userID == "" {
		return c.String(http.StatusUnauthorized, "missing token")
	}

	// Получаем пользователя
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
	}

	// Получаем экземпляры интеграций пользователя
	instances, err := h.repo.ListInstances(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load instances for dashboard")
		return c.String(http.StatusInternalServerError, "Failed to load data")
	}

	// Получаем все доступные шаблоны для пользователя
	templates, err := h.repo.ListTemplates(c.Request().Context(), userID, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load templates for dashboard")
		// Не прерываем выполнение, просто покажем 0
	}

	// Считаем публичные шаблоны
	publicTemplates := 0
	for _, t := range templates {
		if t.IsPublic {
			publicTemplates++
		}
	}

	activeCount := 0
	for _, i := range instances {
		if i.IsActive {
			activeCount++
		}
	}

	stats := map[string]interface{}{
		"total_instances":  len(instances),
		"active_instances": activeCount,
		"total_templates":  len(templates),
		"public_templates": publicTemplates,
	}

	return pages.Dashboard(stats, instances, user).Render(c.Request().Context(), c.Response().Writer)
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

	log.Info().Int("count", len(templates)).Msg("Templates loaded for admin page")
	for _, t := range templates {
		log.Info().Str("id", t.ID).Str("name", t.Name).Str("icon", t.Icon).Msg("Template details")
	}

	return pages.TemplatesAdminPage(templates, user).Render(c.Request().Context(), c.Response().Writer)
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
			Name:          name,
			Icon:          icon,
			Description:   description,
			TemplateText:  templateText,
			IsPublic:      isPublic,
			CreatedBy:     sql.NullString{String: userID, Valid: userID != ""},
			SamplePayload: nil,
		}

		if err := h.repo.CreateTemplate(c.Request().Context(), template); err != nil {
			log.Error().Err(err).Msg("Failed to create template")
			return c.String(http.StatusInternalServerError, "Failed to create template")
		}
		log.Info().Str("id", template.ID).Str("name", name).Msg("Template created")
	}

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

	// Получаем обновленный список шаблонов
	templates, err := h.repo.ListTemplates(c.Request().Context(), userID, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load templates after delete")
		return c.String(http.StatusInternalServerError, "Failed to load templates")
	}

	// Возвращаем только таблицу, а не всю страницу
	return pages.TemplatesAdminTable(templates).Render(c.Request().Context(), c.Response().Writer)
}

// ================ Обработчики для шаблонов (пользователи) ================

// TemplatesUserPage отображает список доступных шаблонов для пользователей
func (h *Handler) TemplatesUserPage(c echo.Context) error {
	userID := getUserIDFromContext(c)
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
	}

	// Показываем публичные шаблоны и шаблоны, созданные пользователем (если он админ)
	templates, err := h.repo.ListTemplates(c.Request().Context(), userID, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load templates")
		return c.String(http.StatusInternalServerError, "Failed to load templates")
	}

	return pages.TemplatesUserPage(templates, user).Render(c.Request().Context(), c.Response().Writer)
}

// InstanceCreatePage отображает форму создания экземпляра из шаблона
func (h *Handler) InstanceCreatePage(c echo.Context) error {
	userID := getUserIDFromContext(c)
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
	}

	templateID := c.Param("id")
	template, err := h.repo.GetTemplateByID(c.Request().Context(), templateID)
	if err != nil {
		return c.String(http.StatusNotFound, "Template not found")
	}

	// Проверяем доступ к шаблону
	if !template.IsPublic && template.CreatedBy.String != userID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	return pages.InstanceCreatePage(template, user).Render(c.Request().Context(), c.Response().Writer)
}

// CreateInstance создает новый экземпляр интеграции
func (h *Handler) CreateInstance(c echo.Context) error {
	userID := getUserIDFromContext(c)

	templateID := c.FormValue("template_id")
	name := c.FormValue("name")
	chatID := c.FormValue("chat_id")
	botToken := c.FormValue("bot_token")

	if name == "" || chatID == "" || botToken == "" {
		return c.String(http.StatusBadRequest, "All fields are required")
	}

	// Проверяем существование и доступность шаблона
	template, err := h.repo.GetTemplateByID(c.Request().Context(), templateID)
	if err != nil {
		return c.String(http.StatusNotFound, "Template not found")
	}

	if !template.IsPublic && template.CreatedBy.String != userID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	// Шифруем токен перед сохранением
	encryptedToken, err := h.encryptor.Encrypt(botToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encrypt bot token")
		return c.String(http.StatusInternalServerError, "Failed to encrypt token")
	}

	instance := &domain.IntegrationInstance{
		TemplateID: templateID,
		UserID:     userID,
		Name:       name,
		ChatID:     chatID,
		BotToken:   encryptedToken,
		IsActive:   true,
	}

	if err := h.repo.CreateInstance(c.Request().Context(), instance); err != nil {
		log.Error().Err(err).Msg("Failed to create instance")
		return c.String(http.StatusInternalServerError, "Failed to create instance")
	}

	log.Info().Str("id", instance.ID).Str("name", name).Msg("Instance created")
	return c.Redirect(http.StatusSeeOther, "/instances")
}

// InstancesListPage отображает список экземпляров пользователя
func (h *Handler) InstancesListPage(c echo.Context) error {
	userID := getUserIDFromContext(c)
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
	}

	instances, err := h.repo.ListInstances(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load instances")
		return c.String(http.StatusInternalServerError, "Failed to load instances")
	}

	return pages.InstancesListPage(instances, user).Render(c.Request().Context(), c.Response().Writer)
}

// TestInstance отправляет тестовое сообщение (без шаблона)
func (h *Handler) TestInstance(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	// Загружаем экземпляр (только для получения токена и chat_id)
	instance, err := h.repo.GetInstanceByID(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Интеграция не найдена")
	}

	// Расшифровываем токен бота
	decryptedToken, err := h.encryptor.Decrypt(instance.BotToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decrypt bot token")
		return c.HTML(http.StatusInternalServerError, `<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">Ошибка расшифровки токена</div>`)
	}

	// Создаём клиент Яндекс.Мессенджера
	yandexClient := yandex.NewClient(decryptedToken)

	// Простое тестовое сообщение (без Liquid)
	testMessage := fmt.Sprintf("🔄 *Тестовое сообщение*\n\nЭкземпляр: *%s*\nВремя: *%s*",
		instance.Name,
		time.Now().Format("02.01.2006 15:04:05"))

	// Отправляем сообщение
	err = yandexClient.SendToChat(c.Request().Context(), instance.ChatID, testMessage, nil)

	if err != nil {
		log.Error().Err(err).Str("instance_id", id).Msg("Test message failed")
		return c.HTML(http.StatusInternalServerError, fmt.Sprintf(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">Ошибка: %s</div>`, err.Error()))
	}

	log.Info().Str("instance_id", id).Msg("Test message sent successfully")
	return c.HTML(http.StatusOK, `<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded">✓ Тестовое сообщение отправлено</div>`)
}

// EditInstanceForm отображает форму редактирования экземпляра
func (h *Handler) EditInstanceForm(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	instance, err := h.repo.GetInstanceWithTemplate(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Instance not found")
	}

	user, _ := h.repo.FindUserByID(c.Request().Context(), userID)
	return pages.InstanceEditPage(instance, user).Render(c.Request().Context(), c.Response().Writer)
}

// UpdateInstance обновляет экземпляр интеграции
func (h *Handler) UpdateInstance(c echo.Context) error {
	log.Info().
		Str("method", c.Request().Method).
		Str("path", c.Request().URL.Path).
		Str("_method", c.FormValue("_method")).
		Str("id", c.Param("id")).
		Msg("🚀 UpdateInstance called")

	id := c.Param("id")
	userID := getUserIDFromContext(c)

	// Загружаем существующий экземпляр
	instance, err := h.repo.GetInstanceWithTemplate(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Instance not found")
	}

	// Обновляем поля экземпляра
	instance.Name = c.FormValue("name")
	instance.ChatID = c.FormValue("chat_id")
	instance.IsActive = c.FormValue("is_active") == "on"

	// Обновляем токен если изменился
	if token := c.FormValue("bot_token"); token != "" && token != "***" {
		encryptedToken, err := h.encryptor.Encrypt(token)
		if err != nil {
			log.Error().Err(err).Msg("Failed to encrypt bot token")
			return c.String(http.StatusInternalServerError, "Failed to encrypt token")
		}
		instance.BotToken = encryptedToken
	}

	// Обновляем шаблон если он изменился
	if templateText := c.FormValue("template_text"); templateText != "" && instance.Template != nil {
		instance.Template.TemplateText = templateText
		if err := h.repo.UpdateTemplate(c.Request().Context(), instance.Template); err != nil {
			log.Error().Err(err).Msg("Failed to update template")
			return c.String(http.StatusInternalServerError, "Failed to update template")
		}
	}

	// Сохраняем изменения в экземпляре
	if err := h.repo.UpdateInstance(c.Request().Context(), instance); err != nil {
		log.Error().Err(err).Msg("Failed to update instance")
		return c.String(http.StatusInternalServerError, "Failed to update instance")
	}

	log.Info().Str("id", id).Msg("Instance updated successfully")

	// Редирект на страницу со списком
	return c.HTML(http.StatusOK, `<script>window.location.href='/instances'</script>`)
}

// DeleteInstance удаляет экземпляр интеграции
func (h *Handler) DeleteInstance(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	if err := h.repo.DeleteInstance(c.Request().Context(), id, userID); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete instance")
		return c.String(http.StatusInternalServerError, "Failed to delete instance")
	}

	log.Info().Str("id", id).Msg("Instance deleted successfully")

	// Редирект на страницу со списком
	return c.HTML(http.StatusOK, `<script>window.location.href='/instances'</script>`)
}

// GetLastWebhook возвращает последний вебхук для экземпляра
func (h *Handler) GetLastWebhook(c echo.Context) error {
	id := c.Param("id")
	userID := getUserIDFromContext(c)

	instance, err := h.repo.GetInstanceByID(c.Request().Context(), id, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "Instance not found")
	}

	if instance.LastWebhookAt == nil {
		return c.HTML(http.StatusOK, `<div class="p-4 text-gray-500">Нет сохранённых запросов</div>`)
	}

	// Форматируем JSON для красивого отображения
	var prettyHeaders, prettyBody bytes.Buffer
	json.Indent(&prettyHeaders, instance.LastWebhookHeaders, "", "  ")
	json.Indent(&prettyBody, instance.LastWebhookBody, "", "  ")

	return c.HTML(http.StatusOK, fmt.Sprintf(`
        <div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50" id="webhook-modal">
            <div class="relative top-20 mx-auto p-5 border w-[800px] shadow-lg rounded-md bg-white">
                <div class="flex justify-between items-center mb-4">
                    <h3 class="text-lg font-medium">Последний запрос (%s)</h3>
                    <button onclick="document.getElementById('webhook-modal').remove()" class="text-gray-500 hover:text-gray-700">✕</button>
                </div>
                <div class="mb-4">
                    <h4 class="font-medium mb-2">Заголовки:</h4>
                    <pre class="bg-gray-50 p-3 rounded text-sm overflow-auto max-h-40">%s</pre>
                </div>
                <div>
                    <h4 class="font-medium mb-2">Тело запроса:</h4>
                    <pre class="bg-gray-50 p-3 rounded text-sm overflow-auto max-h-96 font-mono">%s</pre>
                </div>
            </div>
        </div>
    `,
		instance.LastWebhookAt.Format("02.01.2006 15:04:05"),
		prettyHeaders.String(),
		prettyBody.String(),
	))
}

// CustomInstanceCreatePage отображает форму создания кастомной интеграции
func (h *Handler) CustomInstanceCreatePage(c echo.Context) error {
	userID := getUserIDFromContext(c)
	user, err := h.repo.FindUserByID(c.Request().Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
	}

	return pages.CustomInstanceCreatePage(user).Render(c.Request().Context(), c.Response().Writer)
}

// CreateCustomInstance создает новый экземпляр с кастомным шаблоном
func (h *Handler) CreateCustomInstance(c echo.Context) error {
	userID := getUserIDFromContext(c)
	log.Info().Str("user_id", userID).Msg("CreateCustomInstance called")

	name := c.FormValue("name")
	chatID := c.FormValue("chat_id")
	botToken := c.FormValue("bot_token")
	templateText := c.FormValue("template_text")

	log.Info().
		Str("name", name).
		Str("chat_id", chatID).
		Int("template_len", len(templateText)).
		Msg("Form values received")

	if name == "" || chatID == "" || botToken == "" || templateText == "" {
		log.Error().Msg("Missing required fields")
		return c.String(http.StatusBadRequest, "All fields are required")
	}

	// Создаём приватный шаблон для пользователя
	template := &domain.Template{
		Name:         name + " (кастомный)",
		Icon:         "📝",
		Description:  "Кастомная интеграция",
		TemplateText: templateText,
		IsPublic:     false,
		CreatedBy:    sql.NullString{String: userID, Valid: true},
	}

	if err := h.repo.CreateTemplate(c.Request().Context(), template); err != nil {
		log.Error().Err(err).Msg("Failed to create custom template")
		return c.String(http.StatusInternalServerError, "Failed to create template")
	}
	log.Info().Str("template_id", template.ID).Msg("Template created")

	// Шифруем токен
	encryptedToken, err := h.encryptor.Encrypt(botToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encrypt bot token")
		return c.String(http.StatusInternalServerError, "Failed to encrypt token")
	}

	// Создаём экземпляр
	instance := &domain.IntegrationInstance{
		TemplateID: template.ID,
		UserID:     userID,
		Name:       name,
		ChatID:     chatID,
		BotToken:   encryptedToken,
		IsActive:   true,
	}

	if err := h.repo.CreateInstance(c.Request().Context(), instance); err != nil {
		log.Error().Err(err).Msg("Failed to create instance")
		return c.String(http.StatusInternalServerError, "Failed to create instance")
	}

	log.Info().
		Str("instance_id", instance.ID).
		Str("name", name).
		Msg("Custom instance created successfully")

	return c.Redirect(http.StatusSeeOther, "/instances")
}

// Конец файла - больше ничего не должно быть
