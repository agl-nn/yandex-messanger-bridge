package domain

import "time"

// AlertmanagerWebhook - структура для входящих вебхуков от Alertmanager
// Соответствует формату Prometheus Alertmanager API
type AlertmanagerWebhook struct {
	Version           string            `json:"version" db:"version"`
	GroupKey          string            `json:"groupKey" db:"group_key"`
	TruncatedAlerts   int               `json:"truncatedAlerts" db:"truncated_alerts"`
	Status            string            `json:"status" db:"status"` // firing/resolved
	Receiver          string            `json:"receiver" db:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels" db:"group_labels"`
	CommonLabels      map[string]string `json:"commonLabels" db:"common_labels"`
	CommonAnnotations map[string]string `json:"commonAnnotations" db:"common_annotations"`
	ExternalURL       string            `json:"externalURL" db:"external_url"`
	Alerts            []Alert           `json:"alerts" db:"-"`
}

// Alert - отдельный алерт в группе
type Alert struct {
	Status       string            `json:"status"` // firing/resolved
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// Severity levels
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
	SeverityNone     = "none"
)

// AlertmanagerConfig - конфигурация для обработчика Alertmanager
type AlertmanagerConfig struct {
	MinSeverity  string                 `json:"min_severity"`  // Минимальный уровень severity
	SendResolved bool                   `json:"send_resolved"` // Отправлять resolved алерты
	GroupMode    string                 `json:"group_mode"`    // "single" или "group"
	LabelFilters map[string]string      `json:"label_filters"` // Фильтр по меткам
	Template     string                 `json:"template"`      // Шаблон сообщения
	CustomFields map[string]interface{} `json:"custom_fields"` // Дополнительные настройки
}

// IsSeverityMet проверяет, соответствует ли severity минимальному уровню
func (c *AlertmanagerConfig) IsSeverityMet(severity string) bool {
	if c.MinSeverity == "" {
		return true
	}

	levels := map[string]int{
		"critical": 4,
		"warning":  3,
		"info":     2,
		"none":     1,
	}

	return levels[severity] >= levels[c.MinSeverity]
}

// ShouldSendAlert проверяет, нужно ли отправлять алерт на основе всех фильтров
func (c *AlertmanagerConfig) ShouldSendAlert(alert *Alert) bool {
	// Проверка resolved
	if !c.SendResolved && alert.Status == "resolved" {
		return false
	}

	// Проверка severity
	severity := alert.Labels["severity"]
	if severity == "" {
		severity = alert.Labels["level"]
	}
	if !c.IsSeverityMet(severity) {
		return false
	}

	// Проверка фильтров по меткам
	for k, v := range c.LabelFilters {
		if alert.Labels[k] != v {
			return false
		}
	}

	return true
}
