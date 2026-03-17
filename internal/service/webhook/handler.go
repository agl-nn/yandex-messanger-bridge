// Путь: internal/service/webhook/handler.go
package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/osteele/liquid"
	"github.com/rs/zerolog/log"

	"bytes"
	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
	"yandex-messenger-bridge/internal/yandex"
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

// readBody читает и возвращает тело запроса
func (h *Handler) readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
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

	// Логируем размер тела запроса
	log.Info().
		Str("instance_id", instanceID).
		Int("body_size", len(body)).
		Msg("📦 Webhook body size")

	// Загружаем экземпляр с шаблоном
	instance, err := h.repo.GetInstanceWithTemplate(ctx, instanceID, "")
	if err != nil {
		log.Error().Err(err).Str("id", instanceID).Msg("Instance not found")
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	// Сохраняем последний вебхук
	headers, _ := json.Marshal(r.Header)
	now := time.Now()

	// Логируем размер данных перед сохранением
	log.Info().
		Str("instance_id", instanceID).
		Int("headers_size", len(headers)).
		Int("body_size", len(body)).
		Msg("💾 Saving to database")

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

	// Логируем входящий запрос
	log.Info().
		Str("instance_id", instanceID).
		Str("template", instance.Template.Name).
		Interface("data", data).
		Msg("Processing webhook")

	// Применяем Liquid шаблон
	engine := liquid.NewEngine()
	out, err := engine.ParseAndRenderString(instance.Template.TemplateText, data)
	if err != nil {
		log.Error().Err(err).Str("instance_id", instanceID).Msg("Failed to render template")
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	// Расшифровываем токен бота
	decryptedToken, err := h.encryptor.Decrypt(instance.BotToken)
	if err != nil {
		log.Error().Err(err).Str("instance_id", instanceID).Msg("Failed to decrypt bot token")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Создаём клиент Яндекс.Мессенджера
	yandexClient := yandex.NewClient(decryptedToken)

	// Отправляем сообщение
	err = yandexClient.SendToChat(ctx, instance.ChatID, out, nil)

	if err != nil {
		log.Error().Err(err).Str("instance_id", instanceID).Msg("Failed to send message")
		http.Error(w, "Failed to send", http.StatusInternalServerError)
		return
	}

	log.Info().Str("instance_id", instanceID).Msg("Message sent successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
