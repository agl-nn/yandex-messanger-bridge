// Путь: internal/service/webhook/handler.go
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"yandex-messenger-bridge/internal/domain"
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
