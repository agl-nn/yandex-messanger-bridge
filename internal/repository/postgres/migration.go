// Путь: internal/repository/postgres/migration.go
package postgres

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// RunMigrations выполняет миграции базы данных, если они еще не были выполнены
func RunMigrations(db *sql.DB, dsn string) error {
	// Создаем таблицу для отслеживания миграций, если её нет
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version INT PRIMARY KEY,
            applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Читаем файл миграции
	migrationPath := "migrations/001_initial_schema.sql"
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Проверяем, выполнялась ли уже эта миграция
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if count > 0 {
		// Миграция уже выполнена
		return nil
	}

	// Разделяем на отдельные запросы
	queries := strings.Split(string(content), ";")

	// Выполняем каждый запрос в транзакции
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		_, err := tx.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to execute migration: %s\nError: %w", query, err)
		}
	}

	// Записываем, что миграция выполнена
	_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES (1)")
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}
