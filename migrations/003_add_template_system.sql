-- Добавляем новые поля в существующую таблицу templates
ALTER TABLE templates ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS icon TEXT;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS is_public BOOLEAN DEFAULT false;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Обновляем существующие записи: если name NULL, ставим 'Кастомный шаблон'
UPDATE templates SET name = 'Кастомный шаблон' WHERE name IS NULL;

-- Теперь делаем name NOT NULL
ALTER TABLE templates ALTER COLUMN name SET NOT NULL;

-- Создаём таблицу для экземпляров (инстансов)
CREATE TABLE IF NOT EXISTS integration_instances (
                                                     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID REFERENCES templates(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    bot_token TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    custom_settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- Индексы для быстрого поиска
CREATE INDEX idx_templates_created_by ON templates(created_by);
CREATE INDEX idx_templates_is_public ON templates(is_public);
CREATE INDEX idx_instances_template_id ON integration_instances(template_id);
CREATE INDEX idx_instances_user_id ON integration_instances(user_id);

-- Комментарии к таблицам
COMMENT ON TABLE templates IS 'Шаблоны постоянных интеграций (создаются администраторами)';
COMMENT ON TABLE integration_instances IS 'Экземпляры интеграций, созданные пользователями на основе шаблонов';