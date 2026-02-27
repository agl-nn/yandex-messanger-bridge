package domain

import (
	"time"
	"encoding/json"
)

// User - пользователь системы (оставляем только здесь)
type User struct {
	ID           string    `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	DisplayName  string    `db:"display_name" json:"display_name"`
	Username     string    `db:"username" json:"username"`
	Role         string    `db:"role" json:"role"`
	AuthType     string    `db:"auth_type" json:"auth_type"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// Integration - основная модель интеграции
type Integration struct {
	ID                string                 `db:"id" json:"id"`
	UserID            string                 `db:"user_id" json:"user_id"`
	Name              string                 `db:"name" json:"name"`
	SourceType        string                 `db:"source_type" json:"source_type"`
	SourceConfig      map[string]interface{} `db:"source_config" json:"source_config"`
	DestinationType   string                 `db:"destination_type" json:"destination_type"`
	DestinationConfig DestinationConfig      `db:"destination_config" json:"destination_config"`
	IsActive          bool                   `db:"is_active" json:"is_active"`
	CreatedAt         time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time              `db:"updated_at" json:"updated_at"`
	WebhookURL        string                 `db:"-" json:"webhook_url"`
}

// DestinationConfig - конфигурация назначения
type DestinationConfig struct {
	ChatID   string `json:"chat_id" db:"chat_id"`
	BotToken string `json:"bot_token" db:"bot_token"`
}

// DeliveryLog - лог доставки
type DeliveryLog struct {
	ID             int64           `db:"id" json:"id"`
	IntegrationID  string          `db:"integration_id" json:"integration_id"`
	SourceEventID  string          `db:"source_event_id" json:"source_event_id"`
	RequestPayload json.RawMessage `db:"request_payload" json:"request_payload"`
	ResponseStatus int             `db:"response_status" json:"response_status"`
	ResponseBody   json.RawMessage `db:"response_body" json:"response_body"`
	Error          string          `db:"error" json:"error"`
	DeliveredAt    time.Time       `db:"delivered_at" json:"delivered_at"`
	DurationMS     int             `db:"duration_ms" json:"duration_ms"`
}

// APIKey - ключ для доступа к API
type APIKey struct {
	ID         string     `db:"id" json:"id"`
	UserID     string     `db:"user_id" json:"user_id"`
	KeyHash    string     `db:"key_hash" json:"-"`
	Name       string     `db:"name" json:"name"`
	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	ExpiresAt  *time.Time `db:"expires_at" json:"expires_at"`
}
