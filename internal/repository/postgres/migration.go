package postgres

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// RunMigrations выполняет миграции базы данных
func RunMigrations(db *sql.DB, dsn string) error {
	// Читаем файл миграции
	migrationPath := filepath.Join("migrations", "001_initial_schema.sql")

	content, err := ioutil.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Разделяем на отдельные запросы
	queries := strings.Split(string(content), ";")

	// Выполняем каждый запрос
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to execute migration: %s\nError: %w", query, err)
		}
	}

	return nil
}
