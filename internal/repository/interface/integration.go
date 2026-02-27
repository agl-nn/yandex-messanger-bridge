package _interface

import (
	"context"
	"yandex-messenger-bridge/internal/domain"
)

// IntegrationRepository - интерфейс для работы с интеграциями
type IntegrationRepository interface {
	// Интеграции
	Create(ctx context.Context, integration *domain.Integration) error
	Update(ctx context.Context, integration *domain.Integration) error
	Delete(ctx context.Context, id string, userID string) error
	FindByID(ctx context.Context, id string) (*domain.Integration, error)
	FindByIDAndUser(ctx context.Context, id string, userID string) (*domain.Integration, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.Integration, error)
	FindAll(ctx context.Context) ([]*domain.Integration, error)

	// Логи
	CreateDeliveryLog(ctx context.Context, log *domain.DeliveryLog) error
	GetDeliveryLogs(ctx context.Context, integrationID string, userID string, limit, offset int) ([]*domain.DeliveryLog, int, error)

	// Пользователи
	CreateUser(ctx context.Context, user *domain.User) error
	FindUserByEmail(ctx context.Context, email string) (*domain.User, error)
	FindUserByID(ctx context.Context, id string) (*domain.User, error)

	// API ключи
	CreateAPIKey(ctx context.Context, key *domain.APIKey) error
	FindAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error
	DeleteAPIKey(ctx context.Context, id string, userID string) error
}
