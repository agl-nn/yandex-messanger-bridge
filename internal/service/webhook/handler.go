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

    // Используем отдельный контекст для чтения запроса (таймаут только на чтение)
    readCtx, readCancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer readCancel()
    r = r.WithContext(readCtx)

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

    // Загружаем экземпляр с шаблоном (быстрая операция)
    instance, err := h.repo.GetInstanceWithTemplate(r.Context(), instanceID, "")
    if err != nil {
        log.Error().Err(err).Str("id", instanceID).Msg("Instance not found")
        http.Error(w, "Instance not found", http.StatusNotFound)
        return
    }

    // Сохраняем последний вебхук (быстрая операция)
    headers, _ := json.Marshal(r.Header)
    now := time.Now()

    if err := h.repo.UpdateInstanceLastWebhook(r.Context(), instanceID, headers, body, now); err != nil {
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

    // ========== АСИНХРОННАЯ ОТПРАВКА ==========
    // Отправляем сообщение в фоне, не блокируя ответ клиенту
    go h.sendMessageAsync(instanceID, instance.ChatID, decryptedToken, out)

    // Немедленно возвращаем успешный ответ
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}

// sendMessageAsync асинхронно отправляет сообщение в Яндекс Мессенджер
func (h *Handler) sendMessageAsync(instanceID, chatID, token, message string) {
    // Создаем отдельный контекст с увеличенным таймаутом для отправки
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    yandexClient := yandex.NewClient(token)

    startTime := time.Now()
    err := yandexClient.SendToChat(ctx, chatID, message, nil)
    duration := time.Since(startTime)

    if err != nil {
        log.Error().
            Err(err).
            Str("instance_id", instanceID).
            Dur("duration", duration).
            Msg("❌ Failed to send message asynchronously")

        // Можно добавить retry логику здесь
        h.retrySendAsync(instanceID, chatID, token, message, 1)
    } else {
        log.Info().
            Str("instance_id", instanceID).
            Dur("duration", duration).
            Msg("✅ Message sent successfully asynchronously")
    }
}

// retrySendAsync повторяет отправку при ошибке (опционально)
func (h *Handler) retrySendAsync(instanceID, chatID, token, message string, attempt int) {
    if attempt > h.config.MaxRetries {
        log.Error().
            Str("instance_id", instanceID).
            Int("attempts", attempt-1).
            Msg("❌ Max retries reached, message lost")
        return
    }

    // Экспоненциальная задержка: 2s, 4s, 8s
    delay := time.Duration(1<<uint(attempt)) * time.Second
    time.Sleep(delay)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    yandexClient := yandex.NewClient(token)

    if err := yandexClient.SendToChat(ctx, chatID, message, nil); err != nil {
        log.Error().
            Err(err).
            Str("instance_id", instanceID).
            Int("attempt", attempt+1).
            Msg("❌ Retry failed")
        h.retrySendAsync(instanceID, chatID, token, message, attempt+1)
    } else {
        log.Info().
            Str("instance_id", instanceID).
            Int("attempt", attempt).
            Msg("✅ Message sent successfully on retry")
    }
}