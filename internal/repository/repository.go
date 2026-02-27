package repository

import "context"

type IntegrationRepository interface {
	FindByID(ctx context.Context, id string) (interface{}, error)
	FindByUserID(ctx context.Context, userID string) ([]interface{}, error)
	Create(ctx context.Context, integration interface{}) error
	Update(ctx context.Context, integration interface{}) error
	Delete(ctx context.Context, id string, userID string) error
	GetDeliveryLogs(ctx context.Context, integrationID string, userID string, limit, offset int) ([]interface{}, int, error)
}
