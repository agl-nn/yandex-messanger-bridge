-- Пользователи системы
CREATE TABLE users (
                       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                       email TEXT UNIQUE NOT NULL,
                       password_hash TEXT NOT NULL,
                       role TEXT NOT NULL DEFAULT 'user',
                       created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                       updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- API ключи для доступа к API
CREATE TABLE api_keys (
                          id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                          user_id UUID REFERENCES users(id) ON DELETE CASCADE,
                          key_hash TEXT UNIQUE NOT NULL, -- Храним только хеш
                          name TEXT NOT NULL,
                          last_used_at TIMESTAMP WITH TIME ZONE,
                          created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                          expires_at TIMESTAMP WITH TIME ZONE
);

-- Интеграции
CREATE TABLE integrations (
                              id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                              user_id UUID REFERENCES users(id) ON DELETE CASCADE,
                              name TEXT NOT NULL,
                              source_type TEXT NOT NULL, -- jira, gitlab, alertmanager, grafana
                              source_config JSONB NOT NULL DEFAULT '{}',
                              destination_type TEXT NOT NULL DEFAULT 'yandex_messenger',
                              destination_config JSONB NOT NULL DEFAULT '{}',
                              is_active BOOLEAN DEFAULT true,
                              created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                              updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы для быстрого поиска
CREATE INDEX idx_integrations_user_id ON integrations(user_id);
CREATE INDEX idx_integrations_source_type ON integrations(source_type);
CREATE INDEX idx_integrations_is_active ON integrations(is_active);

-- Логи доставки
CREATE TABLE delivery_logs (
                               id BIGSERIAL PRIMARY KEY,
                               integration_id UUID REFERENCES integrations(id) ON DELETE SET NULL,
                               source_event_id TEXT,
                               request_payload JSONB,
                               response_status INT,
                               response_body JSONB,
                               error TEXT,
                               delivered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                               duration_ms INT
);

-- Индексы для логов
CREATE INDEX idx_delivery_logs_integration_id ON delivery_logs(integration_id);
CREATE INDEX idx_delivery_logs_delivered_at ON delivery_logs(delivered_at);
CREATE INDEX idx_delivery_logs_response_status ON delivery_logs(response_status);

-- Таблица для хранения сессий (если нужно)
CREATE TABLE sessions (
                          id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                          user_id UUID REFERENCES users(id) ON DELETE CASCADE,
                          token TEXT UNIQUE NOT NULL,
                          expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
                          created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Создаем тестового пользователя (пароль: admin123)
INSERT INTO users (email, password_hash, role) VALUES (
                                                          'admin@localhost',
                                                          '$2a$10$YourHashedPasswordHere', -- Заменить на реальный bcrypt хеш
                                                          'admin'
                                                      );