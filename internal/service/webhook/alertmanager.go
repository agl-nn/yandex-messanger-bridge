package webhook

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"yandex-messenger-bridge/internal/domain"
)

// HandleAlertmanager обрабатывает вебхуки от Alertmanager
func (h *Handler) HandleAlertmanager(w http.ResponseWriter, r *http.Request) {
	integrationID := r.PathValue("id")

	// Устанавливаем таймаут
	ctx, cancel := context.WithTimeout(r.Context(), h.config.AlertmanagerTimeout)
	defer cancel()
	r = r.WithContext(ctx)

	// Загружаем интеграцию
	integration, err := h.getIntegrationByID(ctx, integrationID)
	if err != nil {
		log.Error().Err(err).Str("id", integrationID).Msg("Integration not found")
		http.Error(w, "Integration not found", http.StatusNotFound)
		return
	}

	// Читаем тело запроса
	body, err := h.readBody(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read body")
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Парсим Alertmanager webhook
	var alertData domain.AlertmanagerWebhook
	if err := json.Unmarshal(body, &alertData); err != nil {
		log.Error().Err(err).Msg("Failed to parse Alertmanager webhook")
		http.Error(w, "Invalid Alertmanager payload", http.StatusBadRequest)
		return
	}

	// Извлекаем конфигурацию
	alertConfig := &domain.AlertmanagerConfig{}
	if err := mapToStruct(integration.SourceConfig, alertConfig); err != nil {
		log.Error().Err(err).Msg("Failed to parse Alertmanager config")
		// Используем значения по умолчанию
		alertConfig = &domain.AlertmanagerConfig{
			SendResolved: false,
			GroupMode:    "single",
		}
	}

	// Фильтруем алерты (улучшение #5)
	var alertsToSend []domain.Alert
	for _, alert := range alertData.Alerts {
		if alertConfig.ShouldSendAlert(&alert) {
			alertsToSend = append(alertsToSend, alert)
		}
	}

	if len(alertsToSend) == 0 {
		// Нет алертов для отправки
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","filtered":true}`))
		return
	}

	// Формируем сообщение в зависимости от режима группировки
	var message string
	switch alertConfig.GroupMode {
	case "group":
		message = h.formatAlertmanagerGroup(&alertData, alertsToSend, alertConfig)
	default:
		// Отправляем отдельные сообщения или одно сгруппированное
		if len(alertsToSend) == 1 {
			message = h.formatAlertmanagerSingle(&alertsToSend[0], &alertData, alertConfig)
		} else {
			message = h.formatAlertmanagerGroup(&alertData, alertsToSend, alertConfig)
		}
	}

	// Отправляем в Yandex
	if err := h.sendToYandex(ctx, integration, message); err != nil {
		log.Error().Err(err).Msg("Failed to send to Yandex")
		http.Error(w, "Failed to send", http.StatusInternalServerError)
		return
	}

	h.logDelivery(integrationID, alertData, nil)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","alerts_sent":` + string(len(alertsToSend)) + `}`))
}

// formatAlertmanagerSingle форматирует один алерт
func (h *Handler) formatAlertmanagerSingle(alert *domain.Alert, webhook *domain.AlertmanagerWebhook, config *domain.AlertmanagerConfig) string {
	var builder strings.Builder

	// Эмодзи для статуса
	statusEmoji := "🔔"
	if alert.Status == "resolved" {
		statusEmoji = "✅"
	}

	// Эмодзи для severity
	severity := alert.Labels["severity"]
	if severity == "" {
		severity = alert.Labels["level"]
	}
	severityEmoji := map[string]string{
		"critical": "🔥",
		"warning":  "⚠️",
		"info":     "ℹ️",
	}[severity]
	if severityEmoji == "" {
		severityEmoji = "📢"
	}

	// Используем шаблон или дефолтное форматирование
	if config.Template != "" {
		// Заменяем переменные в шаблоне
		msg := config.Template
		msg = strings.ReplaceAll(msg, "{status}", strings.ToUpper(alert.Status))
		msg = strings.ReplaceAll(msg, "{severity}", severity)
		msg = strings.ReplaceAll(msg, "{alertname}", alert.Labels["alertname"])
		msg = strings.ReplaceAll(msg, "{instance}", alert.Labels["instance"])
		msg = strings.ReplaceAll(msg, "{job}", alert.Labels["job"])
		msg = strings.ReplaceAll(msg, "{description}", alert.Annotations["description"])
		msg = strings.ReplaceAll(msg, "{summary}", alert.Annotations["summary"])
		msg = strings.ReplaceAll(msg, "{value}", alert.Annotations["value"])
		return msg
	}

	// Дефолтное форматирование
	builder.WriteString(statusEmoji + " " + severityEmoji + " ")
	builder.WriteString(fmt.Sprintf("*[%s]* ", strings.ToUpper(alert.Status)))

	if name := alert.Labels["alertname"]; name != "" {
		builder.WriteString(fmt.Sprintf("*%s*", name))
	}

	if instance := alert.Labels["instance"]; instance != "" {
		builder.WriteString(fmt.Sprintf(" on `%s`", instance))
	}

	if desc := alert.Annotations["description"]; desc != "" {
		builder.WriteString(fmt.Sprintf("\n📝 %s", desc))
	}

	if value := alert.Annotations["value"]; value != "" {
		builder.WriteString(fmt.Sprintf("\n📊 Current value: %s", value))
	}

	if alert.GeneratorURL != "" {
		builder.WriteString(fmt.Sprintf("\n🔗 [Подробнее](%s)", alert.GeneratorURL))
	}

	return builder.String()
}

// formatAlertmanagerGroup форматирует группу алертов
func (h *Handler) formatAlertmanagerGroup(webhook *domain.AlertmanagerWebhook, alerts []domain.Alert, config *domain.AlertmanagerConfig) string {
	var builder strings.Builder

	// Заголовок группы
	statusEmoji := "🔔"
	if webhook.Status == "resolved" {
		statusEmoji = "✅"
	}

	groupLabels := formatLabels(webhook.GroupLabels)
	builder.WriteString(fmt.Sprintf("%s *[%s] %s*\n",
		statusEmoji,
		strings.ToUpper(webhook.Status),
		groupLabels,
	))

	// Каждый алерт с отступом
	for i, alert := range alerts {
		if i > 0 {
			builder.WriteString("\n---\n")
		}

		severity := alert.Labels["severity"]
		if severity == "" {
			severity = alert.Labels["level"]
		}
		severityEmoji := map[string]string{
			"critical": "🔥",
			"warning":  "⚠️",
			"info":     "ℹ️",
		}[severity]

		builder.WriteString(fmt.Sprintf("%s ", severityEmoji))

		if name := alert.Labels["alertname"]; name != "" {
			builder.WriteString(fmt.Sprintf("*%s*", name))
		}

		if instance := alert.Labels["instance"]; instance != "" {
			builder.WriteString(fmt.Sprintf(" on `%s`", instance))
		}

		if desc := alert.Annotations["description"]; desc != "" {
			builder.WriteString(fmt.Sprintf("\n  📝 %s", desc))
		}
	}

	if len(alerts) > 1 {
		builder.WriteString(fmt.Sprintf("\n\n📊 Всего алертов: %d", len(alerts)))
	}

	return builder.String()
}

// Вспомогательная функция для форматирования меток
func formatLabels(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		if k != "alertname" && k != "severity" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(parts, ", ")
}
