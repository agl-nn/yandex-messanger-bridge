package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"

	"yandex-messenger-bridge/internal/domain"
	repoInterface "yandex-messenger-bridge/internal/repository/interface"
)

// IntegrationRepository - PostgreSQL реализация
type IntegrationRepository struct {
	db *sqlx.DB
}

// NewIntegrationRepository создает новый репозиторий
func NewIntegrationRepository(db *sqlx.DB) repoInterface.IntegrationRepository {
	return &IntegrationRepository{db: db}
}

// Create создает новую интеграцию
func (r *IntegrationRepository) Create(ctx context.Context, integration *domain.Integration) error {
	query := `
        INSERT INTO integrations (id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, created_at, updated_at)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `

	destConfigJSON, err := json.Marshal(integration.DestinationConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal destination config: %w", err)
	}

	sourceConfigJSON, err := json.Marshal(integration.SourceConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal source config: %w", err)
	}

	row := r.db.QueryRowContext(ctx, query,
		integration.UserID,
		integration.Name,
		integration.SourceType,
		sourceConfigJSON,
		integration.DestinationType,
		destConfigJSON,
		integration.IsActive,
	)

	return row.Scan(&integration.ID, &integration.CreatedAt, &integration.UpdatedAt)
}

// Update обновляет интеграцию
func (r *IntegrationRepository) Update(ctx context.Context, integration *domain.Integration) error {
	query := `
        UPDATE integrations 
        SET name = $1, source_config = $2, destination_config = $3, is_active = $4, updated_at = NOW()
        WHERE id = $5 AND user_id = $6
    `

	destConfigJSON, err := json.Marshal(integration.DestinationConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal destination config: %w", err)
	}

	sourceConfigJSON, err := json.Marshal(integration.SourceConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal source config: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		integration.Name,
		sourceConfigJSON,
		destConfigJSON,
		integration.IsActive,
		integration.ID,
		integration.UserID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Delete удаляет интеграцию
func (r *IntegrationRepository) Delete(ctx context.Context, id string, userID string) error {
	query := `DELETE FROM integrations WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// FindByID находит интеграцию по ID
func (r *IntegrationRepository) FindByID(ctx context.Context, id string) (*domain.Integration, error) {
	var integration domain.Integration
	var destConfigJSON []byte
	var sourceConfigJSON []byte

	query := `
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, created_at, updated_at
        FROM integrations
        WHERE id = $1
    `

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&integration.ID,
		&integration.UserID,
		&integration.Name,
		&integration.SourceType,
		&sourceConfigJSON,
		&integration.DestinationType,
		&destConfigJSON,
		&integration.IsActive,
		&integration.CreatedAt,
		&integration.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(destConfigJSON, &integration.DestinationConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal destination config: %w", err)
	}

	if err := json.Unmarshal(sourceConfigJSON, &integration.SourceConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source config: %w", err)
	}

	return &integration, nil
}

// FindByIDAndUser находит интеграцию по ID и пользователю
func (r *IntegrationRepository) FindByIDAndUser(ctx context.Context, id string, userID string) (*domain.Integration, error) {
	var integration domain.Integration
	var destConfigJSON []byte
	var sourceConfigJSON []byte

	query := `
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, created_at, updated_at
        FROM integrations
        WHERE id = $1 AND user_id = $2
    `

	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&integration.ID,
		&integration.UserID,
		&integration.Name,
		&integration.SourceType,
		&sourceConfigJSON,
		&integration.DestinationType,
		&destConfigJSON,
		&integration.IsActive,
		&integration.CreatedAt,
		&integration.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(destConfigJSON, &integration.DestinationConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal destination config: %w", err)
	}

	if err := json.Unmarshal(sourceConfigJSON, &integration.SourceConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source config: %w", err)
	}

	return &integration, nil
}

// FindByUserID находит все интеграции пользователя
func (r *IntegrationRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.Integration, error) {
	var integrations []*domain.Integration

	query := `
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, created_at, updated_at
        FROM integrations
        WHERE user_id = $1
        ORDER BY created_at DESC
    `

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var integration domain.Integration
		var destConfigJSON []byte
		var sourceConfigJSON []byte

		err := rows.Scan(
			&integration.ID,
			&integration.UserID,
			&integration.Name,
			&integration.SourceType,
			&sourceConfigJSON,
			&integration.DestinationType,
			&destConfigJSON,
			&integration.IsActive,
			&integration.CreatedAt,
			&integration.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(destConfigJSON, &integration.DestinationConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal destination config: %w", err)
		}

		if err := json.Unmarshal(sourceConfigJSON, &integration.SourceConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source config: %w", err)
		}

		integrations = append(integrations, &integration)
	}

	return integrations, nil
}

// FindAll находит все интеграции
func (r *IntegrationRepository) FindAll(ctx context.Context) ([]*domain.Integration, error) {
	var integrations []*domain.Integration

	query := `
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, created_at, updated_at
        FROM integrations
        ORDER BY created_at DESC
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var integration domain.Integration
		var destConfigJSON []byte
		var sourceConfigJSON []byte

		err := rows.Scan(
			&integration.ID,
			&integration.UserID,
			&integration.Name,
			&integration.SourceType,
			&sourceConfigJSON,
			&integration.DestinationType,
			&destConfigJSON,
			&integration.IsActive,
			&integration.CreatedAt,
			&integration.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(destConfigJSON, &integration.DestinationConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal destination config: %w", err)
		}

		if err := json.Unmarshal(sourceConfigJSON, &integration.SourceConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source config: %w", err)
		}

		integrations = append(integrations, &integration)
	}

	return integrations, nil
}

// CreateDeliveryLog создает лог доставки
func (r *IntegrationRepository) CreateDeliveryLog(ctx context.Context, log *domain.DeliveryLog) error {
	query := `
        INSERT INTO delivery_logs (integration_id, source_event_id, request_payload, response_status, response_body, error, delivered_at, duration_ms)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id
    `

	return r.db.QueryRowContext(ctx, query,
		log.IntegrationID,
		log.SourceEventID,
		log.RequestPayload,
		log.ResponseStatus,
		log.ResponseBody,
		log.Error,
		log.DeliveredAt,
		log.DurationMS,
	).Scan(&log.ID)
}

// GetDeliveryLogs получает логи доставки
func (r *IntegrationRepository) GetDeliveryLogs(ctx context.Context, integrationID string, userID string, limit, offset int) ([]*domain.DeliveryLog, int, error) {
	var logs []*domain.DeliveryLog
	var total int

	checkQuery := `SELECT COUNT(*) FROM integrations WHERE id = $1 AND user_id = $2`
	var count int
	err := r.db.GetContext(ctx, &count, checkQuery, integrationID, userID)
	if err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return nil, 0, sql.ErrNoRows
	}

	totalQuery := `SELECT COUNT(*) FROM delivery_logs WHERE integration_id = $1`
	err = r.db.GetContext(ctx, &total, totalQuery, integrationID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT id, integration_id, source_event_id, request_payload, response_status, response_body, error, delivered_at, duration_ms
        FROM delivery_logs
        WHERE integration_id = $1
        ORDER BY delivered_at DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := r.db.QueryContext(ctx, query, integrationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var log domain.DeliveryLog
		err := rows.Scan(
			&log.ID,
			&log.IntegrationID,
			&log.SourceEventID,
			&log.RequestPayload,
			&log.ResponseStatus,
			&log.ResponseBody,
			&log.Error,
			&log.DeliveredAt,
			&log.DurationMS,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}

// CreateUser создает пользователя
func (r *IntegrationRepository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
        INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
        VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `

	return r.db.QueryRowContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

// FindUserByEmail находит пользователя по email
func (r *IntegrationRepository) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	query := `
        SELECT id, email, password_hash, role, created_at, updated_at
        FROM users
        WHERE email = $1
    `

	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// FindUserByID находит пользователя по ID
func (r *IntegrationRepository) FindUserByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User

	query := `
        SELECT id, email, password_hash, role, created_at, updated_at
        FROM users
        WHERE id = $1
    `

	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateAPIKey создает API ключ
func (r *IntegrationRepository) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	query := `
        INSERT INTO api_keys (id, user_id, key_hash, name, last_used_at, created_at, expires_at)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW(), $5)
        RETURNING id, created_at
    `

	return r.db.QueryRowContext(ctx, query,
		key.UserID,
		key.KeyHash,
		key.Name,
		key.LastUsedAt,
		key.ExpiresAt,
	).Scan(&key.ID, &key.CreatedAt)
}

// FindAPIKeyByHash находит API ключ по хешу
func (r *IntegrationRepository) FindAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var key domain.APIKey

	query := `
        SELECT id, user_id, key_hash, name, last_used_at, created_at, expires_at
        FROM api_keys
        WHERE key_hash = $1
    `

	err := r.db.GetContext(ctx, &key, query, hash)
	if err != nil {
		return nil, err
	}

	return &key, nil
}

// UpdateAPIKeyLastUsed обновляет время использования ключа
func (r *IntegrationRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	query := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteAPIKey удаляет API ключ
func (r *IntegrationRepository) DeleteAPIKey(ctx context.Context, id string, userID string) error {
	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
