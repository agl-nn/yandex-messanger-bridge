// Путь: internal/repository/postgres/migration.go
package postgres

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// RunMigrations выполняет все миграции, которые ещё не были выполнены
func RunMigrations(db *sql.DB, dsn string) error {
	// Создаем таблицу для отслеживания миграций, если её нет
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version INT PRIMARY KEY,
            name TEXT NOT NULL,
            applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Получаем список выполненных миграций
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return err
		}
		applied[version] = true
	}

	// Читаем все файлы миграций из директории
	files, err := ioutil.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Выполняем каждую миграцию по порядку
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		// Парсим версию из имени файла (например, "001_initial_schema.sql")
		var version int
		_, err := fmt.Sscanf(file.Name(), "%d_", &version)
		if err != nil {
			continue // пропускаем файлы с неправильным именем
		}

		// Если миграция уже выполнена, пропускаем
		if applied[version] {
			continue
		}

		// Читаем файл миграции
		content, err := ioutil.ReadFile(filepath.Join("migrations", file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
		}

		// Выполняем в транзакции
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Разделяем на отдельные запросы
		queries := strings.Split(string(content), ";")
		for _, query := range queries {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}

			_, err := tx.Exec(query)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute migration %s: %s\nError: %w", file.Name(), query, err)
			}
		}

		// Записываем, что миграция выполнена
		_, err = tx.Exec("INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", version, file.Name())
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		// Коммитим транзакцию
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		fmt.Printf("Applied migration: %s\n", file.Name())
	}

	return nil
}
