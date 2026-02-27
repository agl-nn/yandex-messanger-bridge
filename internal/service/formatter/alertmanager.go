package formatter

import (
	"fmt"
	"strings"

	"yandex-messenger-bridge/internal/domain"
)

// AlertmanagerFormatter форматирует сообщения из Alertmanager
type AlertmanagerFormatter struct{}

// NewAlertmanagerFormatter создает новый форматтер
func NewAlertmanagerFormatter() *AlertmanagerFormatter {
	return &AlertmanagerFormatter{}
}

// Format форматирует алерт
func (f *AlertmanagerFormatter) Format(alert *domain.Alert, webhook *domain.AlertmanagerWebhook, config *domain.AlertmanagerConfig) string {
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
	if config != nil && config.Template != "" {
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

// FormatGroup форматирует группу алертов
func (f *AlertmanagerFormatter) FormatGroup(webhook *domain.AlertmanagerWebhook, alerts []domain.Alert, config *domain.AlertmanagerConfig) string {
	var builder strings.Builder

	// Заголовок группы
	statusEmoji := "🔔"
	if webhook.Status == "resolved" {
		statusEmoji = "✅"
	}

	groupLabels := f.formatLabels(webhook.GroupLabels)
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

// formatLabels форматирует метки для отображения
func (f *AlertmanagerFormatter) formatLabels(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		if k != "alertname" && k != "severity" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(parts, ", ")
}
