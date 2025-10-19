package config

import (
	"os"
	"strconv"

	"go.uber.org/zap"
)

// Config holds all configuration for the application.
type Config struct {
	AppEnv                string
	DBPath                string
	DBDriver              string
	RedisAddr             string
	GRPCPort              int
	GRPCReflectionEnabled bool
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() *Config {
	portStr := getEnv("GRPC_PORT", "50051")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 50051
	}

	reflectionStr := getEnv("GRPC_REFLECTION_ENABLED", "false")
	reflection, err := strconv.ParseBool(reflectionStr)
	if err != nil {
		reflection = false
	}

	return &Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		DBPath:                getEnv("DB_PATH", "./data/database.db"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		DBDriver:              getEnv("DB_DRIVER", "sqlite3"),
		GRPCPort:              port,
		GRPCReflectionEnabled: reflection,
	}
}

// NewLogger creates a new Zap logger based on the config.
func NewLogger(cfg *Config) (*zap.Logger, error) {
	if cfg.AppEnv == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
