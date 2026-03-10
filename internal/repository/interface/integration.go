// Путь: internal/repository/interface/integration.go
package _interface

import (
	"context"
	"encoding/json"
	"time"
	"yandex-messenger-bridge/internal/domain"
)

// IntegrationRepository - интерфейс для работы с интеграциями
type IntegrationRepository interface {
	// Интеграции (старые)
	Create(ctx context.Context, integration *domain.Integration) error
	Update(ctx context.Context, integration *domain.Integration) error
	Delete(ctx context.Context, id string, userID string) error
	FindByID(ctx context.Context, id string) (*domain.Integration, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.Integration, error)
	FindAll(ctx context.Context) ([]*domain.Integration, error)
	FindByIDAndUser(ctx context.Context, id string, userID string) (*domain.Integration, error)

	// Логи
	//CreateDeliveryLog(ctx context.Context, log *domain.DeliveryLog) error
	//GetDeliveryLogs(ctx context.Context, integrationID string, userID string, limit, offset int) ([]*domain.DeliveryLog, int, error)

	// Пользователи
	CreateUser(ctx context.Context, user *domain.User) error
	FindUserByEmail(ctx context.Context, email string) (*domain.User, error)
	FindUserByID(ctx context.Context, id string) (*domain.User, error)

	// API ключи
	CreateAPIKey(ctx context.Context, key *domain.APIKey) error
	FindAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error
	DeleteAPIKey(ctx context.Context, id string, userID string) error

	// ================ НОВЫЕ МЕТОДЫ ДЛЯ ШАБЛОНОВ ================

	// Шаблоны
	CreateTemplate(ctx context.Context, template *domain.Template) error
	UpdateTemplate(ctx context.Context, template *domain.Template) error
	DeleteTemplate(ctx context.Context, id string) error
	GetTemplateByID(ctx context.Context, id string) (*domain.Template, error)
	ListTemplates(ctx context.Context, userID string, includePublic bool) ([]*domain.Template, error)

	// Экземпляры
	CreateInstance(ctx context.Context, instance *domain.IntegrationInstance) error
	UpdateInstance(ctx context.Context, instance *domain.IntegrationInstance) error
	DeleteInstance(ctx context.Context, id string, userID string) error
	GetInstanceByID(ctx context.Context, id string, userID string) (*domain.IntegrationInstance, error)
	ListInstances(ctx context.Context, userID string) ([]*domain.IntegrationInstance, error)
	GetInstanceWithTemplate(ctx context.Context, id string, userID string) (*domain.IntegrationInstance, error)

	// Методы для обратной совместимости
	GetTemplateByIntegrationID(ctx context.Context, integrationID string) (*domain.Template, error)
	FindWithTemplate(ctx context.Context, integrationID string) (*domain.Integration, *domain.Template, error)
	// GetInstanceByIDPublic получает экземпляр по ID без проверки user_id (для вебхуков)
	GetInstanceByIDPublic(ctx context.Context, id string) (*domain.IntegrationInstance, error)
	// UpdateInstanceLastWebhook обновляет поля последнего вебхука
	UpdateInstanceLastWebhook(ctx context.Context, instanceID string, headers, body json.RawMessage, lastAt time.Time) error
	// Управление пользователями
	ListUsers(ctx context.Context) ([]*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	ChangePassword(ctx context.Context, userID string, newPasswordHash string) error
	DeleteUser(ctx context.Context, id string) error
}
