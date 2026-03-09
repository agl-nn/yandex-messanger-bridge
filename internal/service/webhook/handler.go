// Путь: internal/service/webhook/handler.go
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"strings"

	"github.com/osteele/liquid"
	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/internal/domain"
	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/yandex"
	"bytes"
)

// Config - конфигурация вебхук обработчика
type Config struct {
	GitLabTimeout       time.Duration
	AlertmanagerTimeout time.Duration
	JiraTimeout         time.Duration
	MaxRetries          int
}

// Handler - обработчик вебхуков
type Handler struct {
	repo      _interface.IntegrationRepository
	yandex    *yandex.Client
	encryptor *encryption.Encryptor
	config    Config
}

// NewHandler создает новый обработчик
func NewHandler(
	repo _interface.IntegrationRepository,
	yandex *yandex.Client,
	encryptor *encryption.Encryptor,
	config Config,
) *Handler {
	return &Handler{
		repo:      repo,
		yandex:    yandex,
		encryptor: encryptor,
		config:    config,
	}
}

// logDelivery логирует попытку доставки
func (h *Handler) logDelivery(integrationID string, payload interface{}, err error) {
	logEntry := log.Info()
	if err != nil {
		logEntry = log.Error().Err(err)
	}

	logEntry.Str("integration_id", integrationID).
		Interface("payload", payload).
		Time("timestamp", time.Now()).
		Msg("Webhook delivery")
}

// getIntegrationByID загружает интеграцию и расшифровывает токен
func (h *Handler) getIntegrationByID(ctx context.Context, id string) (*domain.Integration, error) {
	integration, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("integration not found: %w", err)
	}

	if !integration.IsActive {
		return nil, fmt.Errorf("integration is inactive")
	}

	decrypted, err := h.encryptor.Decrypt(integration.DestinationConfig.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt bot token: %w", err)
	}
	integration.DestinationConfig.BotToken = decrypted

	return integration, nil
}

// sendToYandex отправляет сообщение в Yandex Messenger
func (h *Handler) sendToYandex(ctx context.Context, integration *domain.Integration, message string) error {
	client := yandex.NewClient(integration.DestinationConfig.BotToken)
	return client.SendToChat(ctx, integration.DestinationConfig.ChatID, message, nil)
}

// readBody читает и возвращает тело запроса
func (h *Handler) readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

// mapToStruct конвертирует map в struct
func mapToStruct(m map[string]interface{}, s interface{}) error {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, s)
}

// retrySend повторяет отправку с экспоненциальной задержкой
func (h *Handler) retrySend(integration *domain.Integration, message string, attempt int) {
	if attempt >= h.config.MaxRetries {
		log.Error().Int("attempts", attempt).Msg("Max retries reached")
		return
	}

	delay := time.Duration(1<<uint(attempt)) * time.Second
	time.Sleep(delay)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.sendToYandex(ctx, integration, message); err != nil {
		log.Error().Err(err).Int("attempt", attempt+1).Msg("Retry failed")
		h.retrySend(integration, message, attempt+1)
	}
}

// HandleInstanceWebhook обрабатывает вебхуки для экземпляров интеграций
func (h *Handler) HandleInstanceWebhook(w http.ResponseWriter, r *http.Request) {
	// Получаем ID
	instanceID := r.PathValue("id")
	// Если не сработало, берём из URL вручную
	if instanceID == "" {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) > 0 {
			instanceID = pathParts[len(pathParts)-1]
		}
	}
	log.Info().
		Str("instance_id", instanceID).
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Msg("🔍 Webhook received")

	if instanceID == "" {
		log.Error().Msg("Empty instance ID")
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	r = r.WithContext(ctx)

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read body")
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Загружаем экземпляр с шаблоном
	instance, err := h.repo.GetInstanceWithTemplate(ctx, instanceID, "")
	if err != nil {
		log.Error().Err(err).Str("id", instanceID).Msg("Instance not found")
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	// Сохраняем последний вебхук (ОДИН РАЗ)
	headers, _ := json.Marshal(r.Header)
	now := time.Now()

	if err := h.repo.UpdateInstanceLastWebhook(ctx, instanceID, headers, body, now); err != nil {
		log.Error().Err(err).Msg("Failed to save last webhook")
		// Не прерываем обработку
	}

	// Проверяем, активна ли интеграция
	if !instance.IsActive {
		log.Warn().Str("id", instanceID).Msg("Instance is inactive")
		http.Error(w, "Instance is inactive", http.StatusForbidden)
		return
	}

	// Парсим JSON в map для Liquid
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Error().Err(err).Msg("Failed to parse JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Логируем входящий запрос (для отладки)
	log.Info().
		Str("instance_id", instanceID).
		Str("template", instance.Template.Name).
		Interface("data", data).
		Msg("Processing webhook")

	// Применяем Liquid шаблон
	engine := liquid.NewEngine()
	out, err := engine.ParseAndRenderString(instance.Template.TemplateText, data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to render template")
		h.saveDeliveryLog(ctx, instanceID, body, 0, nil, fmt.Errorf("template error: %w", err))
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	// Расшифровываем токен бота
	decryptedToken, err := h.encryptor.Decrypt(instance.BotToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decrypt bot token")
		h.saveDeliveryLog(ctx, instanceID, body, 0, nil, fmt.Errorf("failed to decrypt token: %w", err))
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Создаём клиент Яндекс.Мессенджера
	yandexClient := yandex.NewClient(decryptedToken)

	// Отправляем сообщение
	err = yandexClient.SendToChat(ctx, instance.ChatID, out, nil)

	// Сохраняем лог доставки
	status := 200
	if err != nil {
		status = 500
	}
	h.saveDeliveryLog(ctx, instanceID, body, status, []byte(out), err)

	if err != nil {
		log.Error().Err(err).Str("instance_id", instanceID).Msg("Failed to send message")
		http.Error(w, "Failed to send", http.StatusInternalServerError)
		return
	}

	log.Info().Str("instance_id", instanceID).Msg("Message sent successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// saveDeliveryLog сохраняет лог доставки
func (h *Handler) saveDeliveryLog(ctx context.Context, instanceID string, request []byte, status int, response []byte, err error) {
	// Проверяем, что request - валидный JSON
	var requestJSON json.RawMessage
	if json.Valid(request) {
		requestJSON = request
	} else {
		// Если не JSON, оборачиваем в JSON-строку
		invalidJSON, _ := json.Marshal(string(request))
		requestJSON = invalidJSON
	}

	// Проверяем response
	var responseJSON json.RawMessage
	if len(response) > 0 && json.Valid(response) {
		responseJSON = response
	} else if len(response) > 0 {
		// Если не JSON, оборачиваем в JSON-строку
		invalidJSON, _ := json.Marshal(string(response))
		responseJSON = invalidJSON
	}

	logEntry := &domain.DeliveryLog{
		IntegrationID:  instanceID,
		SourceEventID:  "",
		RequestPayload: requestJSON,
		ResponseStatus: status,
		ResponseBody:   responseJSON,
		DeliveredAt:    time.Now(),
		DurationMS:     0,
	}

	if err != nil {
		logEntry.Error = err.Error()
	}

	if err := h.repo.CreateDeliveryLog(ctx, logEntry); err != nil {
		log.Error().Err(err).Msg("Failed to save delivery log")
	}
}
