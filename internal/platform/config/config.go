package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        string
	AutoMigrate     bool
	ShutdownTimeout time.Duration
	OutboxInterval  time.Duration
	OutboxBatchSize int
	StaffAPIKey     string
}

func Load() (Config, error) {
	cfg := Config{
		Env:             stringEnv("APP_ENV", "development"),
		HTTPAddr:        stringEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		LogLevel:        stringEnv("LOG_LEVEL", "info"),
		AutoMigrate:     boolEnv("AUTO_MIGRATE", true),
		ShutdownTimeout: 15 * time.Second,
		OutboxInterval:  2 * time.Second,
		OutboxBatchSize: 20,
		StaffAPIKey:     os.Getenv("STAFF_API_KEY"),
	}

	if port := os.Getenv("PORT"); port != "" {
		cfg.HTTPAddr = ":" + port
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}

	if cfg.StaffAPIKey == "" {
		return Config{}, fmt.Errorf("config: STAFF_API_KEY is required")
	}
	return cfg, nil
}

func stringEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
