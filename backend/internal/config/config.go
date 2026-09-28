package config

import (
	"errors"
	"os"
)

const (
	defaultHTTPAddr = ":8080"
	defaultAppEnv   = "local"

	envDatabaseURL = "DATABASE_URL"
	envHTTPAddr    = "BACKEND_HTTP_ADDR"
	envAppEnv      = "APP_ENV"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	AppEnv      string
}

func Load() (Config, error) {
	databaseURL := os.Getenv(envDatabaseURL)
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return Config{
		DatabaseURL: databaseURL,
		HTTPAddr:    getEnv(envHTTPAddr, defaultHTTPAddr),
		AppEnv:      getEnv(envAppEnv, defaultAppEnv),
	}, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
