package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"

	"yandex-messenger-bridge/internal/domain"
	repoInterface "yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
)

// IntegrationRepository - PostgreSQL реализация
type IntegrationRepository struct {
	db        *sqlx.DB
	encryptor *encryption.Encryptor
}

// NewIntegrationRepository создает новый репозиторий
func NewIntegrationRepository(db *sqlx.DB, encryptor *encryption.Encryptor) repoInterface.IntegrationRepository {
	return &IntegrationRepository{
		db:        db,
		encryptor: encryptor,
	}
}

// ================ Методы для интеграций (старые) ================

// Create создает новую интеграцию
func (r *IntegrationRepository) Create(ctx context.Context, integration *domain.Integration) error {
	query := `
        INSERT INTO integrations (id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, is_custom, template_id, created_at, updated_at)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
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
		integration.IsCustom,
		integration.TemplateID,
	)

	return row.Scan(&integration.ID, &integration.CreatedAt, &integration.UpdatedAt)
}

// Update обновляет интеграцию
func (r *IntegrationRepository) Update(ctx context.Context, integration *domain.Integration) error {
	query := `
        UPDATE integrations 
        SET name = $1, source_config = $2, destination_config = $3, is_active = $4, is_custom = $5, template_id = $6, updated_at = NOW()
        WHERE id = $7 AND user_id = $8
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
		integration.IsCustom,
		integration.TemplateID,
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
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, is_custom, template_id, created_at, updated_at
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
		&integration.IsCustom,
		&integration.TemplateID,
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
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, is_custom, template_id, created_at, updated_at
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
		&integration.IsCustom,
		&integration.TemplateID,
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
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, is_custom, template_id, created_at, updated_at
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
			&integration.IsCustom,
			&integration.TemplateID,
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
        SELECT id, user_id, name, source_type, source_config, destination_type, destination_config, is_active, is_custom, template_id, created_at, updated_at
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
			&integration.IsCustom,
			&integration.TemplateID,
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

// ================ МЕТОДЫ ДЛЯ ШАБЛОНОВ ================

// CreateTemplate создает новый шаблон
func (r *IntegrationRepository) CreateTemplate(ctx context.Context, template *domain.Template) error {
	query := `
        INSERT INTO templates (id, name, description, icon, template_text, is_public, created_by, sample_payload, created_at, updated_at)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `

	// Передаём 7 параметров
	return r.db.QueryRowContext(ctx, query,
		template.Name,
		template.Description,
		template.Icon,
		template.TemplateText,
		template.IsPublic,
		template.CreatedBy,
		nil, // ← 7-й параметр - sample_payload = NULL
	).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)
}

// UpdateTemplate обновляет шаблон
func (r *IntegrationRepository) UpdateTemplate(ctx context.Context, template *domain.Template) error {
	query := `
        UPDATE templates 
        SET name = $1, description = $2, icon = $3, template_text = $4, 
            is_public = $5, updated_at = NOW()
        WHERE id = $6
    `

	result, err := r.db.ExecContext(ctx, query,
		template.Name,
		template.Description,
		template.Icon,
		template.TemplateText,
		template.IsPublic,
		template.ID,
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

// DeleteTemplate удаляет шаблон
func (r *IntegrationRepository) DeleteTemplate(ctx context.Context, id string) error {
	query := `DELETE FROM templates WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
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

// GetTemplateByID получает шаблон по ID
func (r *IntegrationRepository) GetTemplateByID(ctx context.Context, id string) (*domain.Template, error) {
	var template domain.Template
	var samplePayload []byte
	var integrationID sql.NullString

	query := `
        SELECT id, name, description, icon, template_text, is_public, created_by, sample_payload, created_at, updated_at, integration_id
        FROM templates
        WHERE id = $1
    `

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.Icon,
		&template.TemplateText,
		&template.IsPublic,
		&template.CreatedBy,
		&samplePayload,
		&template.CreatedAt,
		&template.UpdatedAt,
		&integrationID,
	)
	if err != nil {
		return nil, err
	}

	if samplePayload != nil {
		template.SamplePayload = samplePayload
	}

	if integrationID.Valid {
		template.IntegrationID = &integrationID.String
	}

	return &template, nil
}

// ListTemplates возвращает список доступных шаблонов
func (r *IntegrationRepository) ListTemplates(ctx context.Context, userID string, includePublic bool) ([]*domain.Template, error) {
	var templates []*domain.Template

	query := `
        SELECT id, name, description, icon, template_text, is_public, created_by, sample_payload, created_at, updated_at, integration_id
        FROM templates
        WHERE created_by = $1 OR (is_public = true AND $2 = true)
        ORDER BY name
    `

	rows, err := r.db.QueryContext(ctx, query, userID, includePublic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var template domain.Template
		var samplePayload []byte
		var integrationID sql.NullString

		err := rows.Scan(
			&template.ID,
			&template.Name,
			&template.Description,
			&template.Icon,
			&template.TemplateText,
			&template.IsPublic,
			&template.CreatedBy,
			&samplePayload,
			&template.CreatedAt,
			&template.UpdatedAt,
			&integrationID,
		)
		if err != nil {
			return nil, err
		}

		if samplePayload != nil {
			template.SamplePayload = samplePayload
		}

		if integrationID.Valid {
			template.IntegrationID = &integrationID.String
		}

		templates = append(templates, &template)
	}

	return templates, nil
}

// ================ МЕТОДЫ ДЛЯ ЭКЗЕМПЛЯРОВ ================

// CreateInstance создает новый экземпляр интеграции
func (r *IntegrationRepository) CreateInstance(ctx context.Context, instance *domain.IntegrationInstance) error {
	// Шифруем токен перед сохранением
	encryptedToken, err := r.encryptor.Encrypt(instance.BotToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt bot token: %w", err)
	}

	customSettingsJSON, err := json.Marshal(instance.CustomSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal custom settings: %w", err)
	}

	query := `
        INSERT INTO integration_instances (id, template_id, user_id, name, chat_id, bot_token, is_active, custom_settings, created_at, updated_at)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `

	return r.db.QueryRowContext(ctx, query,
		instance.TemplateID,
		instance.UserID,
		instance.Name,
		instance.ChatID,
		encryptedToken,
		instance.IsActive,
		customSettingsJSON,
	).Scan(&instance.ID, &instance.CreatedAt, &instance.UpdatedAt)
}

// UpdateInstance обновляет экземпляр
func (r *IntegrationRepository) UpdateInstance(ctx context.Context, instance *domain.IntegrationInstance) error {
	customSettingsJSON, err := json.Marshal(instance.CustomSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal custom settings: %w", err)
	}

	query := `
        UPDATE integration_instances 
        SET name = $1, chat_id = $2, is_active = $3, custom_settings = $4, updated_at = NOW()
        WHERE id = $5 AND user_id = $6
    `

	result, err := r.db.ExecContext(ctx, query,
		instance.Name,
		instance.ChatID,
		instance.IsActive,
		customSettingsJSON,
		instance.ID,
		instance.UserID,
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

	// Если токен изменился, обновляем отдельно
	if instance.BotToken != "" && instance.BotToken != "***" {
		encryptedToken, err := r.encryptor.Encrypt(instance.BotToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt bot token: %w", err)
		}

		_, err = r.db.ExecContext(ctx, "UPDATE integration_instances SET bot_token = $1 WHERE id = $2", encryptedToken, instance.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteInstance удаляет экземпляр
func (r *IntegrationRepository) DeleteInstance(ctx context.Context, id string, userID string) error {
	query := `DELETE FROM integration_instances WHERE id = $1 AND user_id = $2`

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

// GetInstanceByID получает экземпляр по ID
func (r *IntegrationRepository) GetInstanceByID(ctx context.Context, id string, userID string) (*domain.IntegrationInstance, error) {
	var instance domain.IntegrationInstance
	var encryptedToken string
	var customSettings []byte
	var lastHeaders, lastBody []byte
	var lastAt sql.NullTime

	query := `
        SELECT id, template_id, user_id, name, chat_id, bot_token, is_active, custom_settings,
               last_webhook_headers, last_webhook_body, last_webhook_at,
               created_at, updated_at
        FROM integration_instances
        WHERE id = $1 AND user_id = $2
    `

	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&instance.ID,
		&instance.TemplateID,
		&instance.UserID,
		&instance.Name,
		&instance.ChatID,
		&encryptedToken,
		&instance.IsActive,
		&customSettings,
		&lastHeaders,
		&lastBody,
		&lastAt,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Расшифровываем токен
	decryptedToken, err := r.encryptor.Decrypt(encryptedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt bot token: %w", err)
	}
	instance.BotToken = decryptedToken

	if customSettings != nil {
		if err := json.Unmarshal(customSettings, &instance.CustomSettings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom settings: %w", err)
		}
	}

	if lastHeaders != nil {
		instance.LastWebhookHeaders = lastHeaders
	}
	if lastBody != nil {
		instance.LastWebhookBody = lastBody
	}
	if lastAt.Valid {
		instance.LastWebhookAt = &lastAt.Time
	}

	return &instance, nil
}

// ListInstances возвращает список экземпляров пользователя
func (r *IntegrationRepository) ListInstances(ctx context.Context, userID string) ([]*domain.IntegrationInstance, error) {
	var instances []*domain.IntegrationInstance

	query := `
        SELECT i.id, i.template_id, i.user_id, i.name, i.chat_id, i.is_active, i.custom_settings, i.created_at, i.updated_at,
               t.id as template_id, t.name as template_name, t.icon, t.description, t.template_text
        FROM integration_instances i
        LEFT JOIN templates t ON i.template_id = t.id
        WHERE i.user_id = $1
        ORDER BY i.created_at DESC
    `

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var instance domain.IntegrationInstance
		var template domain.Template
		var customSettings []byte
		var templateID, templateName, templateIcon, templateDescription, templateText sql.NullString

		err := rows.Scan(
			&instance.ID,
			&instance.TemplateID,
			&instance.UserID,
			&instance.Name,
			&instance.ChatID,
			&instance.IsActive,
			&customSettings,
			&instance.CreatedAt,
			&instance.UpdatedAt,
			&templateID,
			&templateName,
			&templateIcon,
			&templateDescription,
			&templateText,
		)
		if err != nil {
			return nil, err
		}

		if customSettings != nil {
			if err := json.Unmarshal(customSettings, &instance.CustomSettings); err != nil {
				return nil, fmt.Errorf("failed to unmarshal custom settings: %w", err)
			}
		}

		// Заполняем шаблон, если он есть
		if templateID.Valid {
			template.ID = templateID.String
			template.Name = templateName.String
			if templateIcon.Valid {
				template.Icon = templateIcon.String
			}
			if templateDescription.Valid {
				template.Description = templateDescription.String
			}
			if templateText.Valid {
				template.TemplateText = templateText.String
			}
			instance.Template = &template
		}

		instances = append(instances, &instance)
	}

	return instances, nil
}

// GetInstanceByIDPublic получает экземпляр по ID без проверки user_id (для вебхуков)
func (r *IntegrationRepository) GetInstanceByIDPublic(ctx context.Context, id string) (*domain.IntegrationInstance, error) {
	var instance domain.IntegrationInstance
	var encryptedToken string
	var customSettings []byte

	query := `
        SELECT id, template_id, user_id, name, chat_id, bot_token, is_active, custom_settings, created_at, updated_at
        FROM integration_instances
        WHERE id = $1
    `

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&instance.ID,
		&instance.TemplateID,
		&instance.UserID,
		&instance.Name,
		&instance.ChatID,
		&encryptedToken,
		&instance.IsActive,
		&customSettings,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Расшифровываем токен
	decryptedToken, err := r.encryptor.Decrypt(encryptedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt bot token: %w", err)
	}
	instance.BotToken = decryptedToken

	if customSettings != nil {
		if err := json.Unmarshal(customSettings, &instance.CustomSettings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom settings: %w", err)
		}
	}

	return &instance, nil
}

// GetInstanceWithTemplate получает экземпляр вместе с шаблоном (обновлённая версия)
func (r *IntegrationRepository) GetInstanceWithTemplate(ctx context.Context, id string, userID string) (*domain.IntegrationInstance, error) {
	var instance *domain.IntegrationInstance
	var err error

	if userID == "" {
		// Публичный доступ (вебхуки)
		instance, err = r.GetInstanceByIDPublic(ctx, id)
	} else {
		// Доступ пользователя
		instance, err = r.GetInstanceByID(ctx, id, userID)
	}

	if err != nil {
		return nil, err
	}

	// Загружаем шаблон
	template, err := r.GetTemplateByID(ctx, instance.TemplateID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		instance.Template = template
	}

	return instance, nil
}

// ================ МЕТОДЫ ДЛЯ ОБРАТНОЙ СОВМЕСТИМОСТИ ================

// GetTemplateByIntegrationID получает шаблон по ID интеграции (старый метод)
func (r *IntegrationRepository) GetTemplateByIntegrationID(ctx context.Context, integrationID string) (*domain.Template, error) {
	var template domain.Template
	var samplePayload []byte

	query := `
        SELECT id, name, description, icon, template_text, is_public, created_by, sample_payload, created_at, updated_at, integration_id
        FROM templates
        WHERE integration_id = $1
    `

	err := r.db.QueryRowContext(ctx, query, integrationID).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.Icon,
		&template.TemplateText,
		&template.IsPublic,
		&template.CreatedBy,
		&samplePayload,
		&template.CreatedAt,
		&template.UpdatedAt,
		&template.IntegrationID,
	)
	if err != nil {
		return nil, err
	}

	if samplePayload != nil {
		template.SamplePayload = samplePayload
	}

	return &template, nil
}

// FindWithTemplate загружает интеграцию вместе с шаблоном (старый метод)
func (r *IntegrationRepository) FindWithTemplate(ctx context.Context, integrationID string) (*domain.Integration, *domain.Template, error) {
	// Сначала загружаем интеграцию
	integration, err := r.FindByID(ctx, integrationID)
	if err != nil {
		return nil, nil, err
	}

	// Если интеграция кастомная, загружаем шаблон
	var template *domain.Template
	if integration.IsCustom {
		tpl, err := r.GetTemplateByIntegrationID(ctx, integrationID)
		if err != nil && err != sql.ErrNoRows {
			return nil, nil, err
		}
		if err == nil {
			template = tpl
		}
	}

	return integration, template, nil
}

// UpdateInstanceLastWebhook обновляет поля последнего вебхука
func (r *IntegrationRepository) UpdateInstanceLastWebhook(ctx context.Context, instanceID string, headers, body json.RawMessage, lastAt time.Time) error {
	query := `
        UPDATE integration_instances 
        SET last_webhook_headers = $1, last_webhook_body = $2, last_webhook_at = $3
        WHERE id = $4
    `
	_, err := r.db.ExecContext(ctx, query, headers, body, lastAt, instanceID)
	return err
}
