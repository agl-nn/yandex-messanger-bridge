-- Удаляем таблицу delivery_logs
DROP TABLE IF EXISTS delivery_logs;

-- Можно также удалить связанные индексы, если они были созданы отдельно
-- (но при удалении таблицы они удалятся автоматически)

COMMENT ON TABLE delivery_logs IS NULL;