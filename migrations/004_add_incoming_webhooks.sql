-- Добавляем поля для хранения последнего вебхука прямо в таблицу экземпляров
ALTER TABLE integration_instances
    ADD COLUMN IF NOT EXISTS last_webhook_headers JSONB,
    ADD COLUMN IF NOT EXISTS last_webhook_body JSONB,
    ADD COLUMN IF NOT EXISTS last_webhook_at TIMESTAMP WITH TIME ZONE;

-- Индекс для быстрой сортировки по времени (может пригодиться)
CREATE INDEX IF NOT EXISTS idx_instances_last_webhook_at ON integration_instances(last_webhook_at);