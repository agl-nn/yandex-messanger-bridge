-- Добавляем поле must_change_password в таблицу users, если его нет
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN DEFAULT false;

-- Создаем или обновляем админа по умолчанию (пароль: admin)
-- Хеш пароля сгенерирован с помощью bcrypt (стоимость 10)
INSERT INTO users (id, email, password_hash, role, must_change_password, created_at, updated_at)
VALUES (
           gen_random_uuid(),
           'admin@localhost',
           '$2a$10$N9qo8uLOickgx2ZMRZoMy.MrZ4KhJfjb3TXBz3VPzQvQg9QzYqXKu', -- hash for 'admin'
           'admin',
           true,
           NOW(),
           NOW()
       )
    ON CONFLICT (email) DO UPDATE SET
       password_hash = EXCLUDED.password_hash,
       role = 'admin',
       must_change_password = true,
       updated_at = NOW();