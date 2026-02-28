package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Port          string
	DatabaseDSN   string
	JWTSecret     string
	BaseURL       string
	EncryptionKey string
}

func Load() *Config {
	viper.AutomaticEnv()

	return &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseDSN:   getEnv("DATABASE_DSN", "postgres://integrator:secret@localhost:5432/integrator?sslmode=disable"),
		JWTSecret:     getEnv("JWT_SECRET", "your-secret-key"),
		BaseURL:       getEnv("BASE_URL", "http://localhost:8080"),
		EncryptionKey: getEnv("ENCRYPTION_KEY", "32-byte-key-for-aes-256-encryption"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := viper.GetString(key); value != "" {
		return value
	}
	return defaultValue
}
