package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env               string
	LogLevel          string
	HTTPPort          string
	GRPCPort          string
	DBDriver          string
	DBDSN             string
	DLQPath           string
	DLQReplayInterval string

	// Ingestion Filtering
	IngestMinSeverity      string
	IngestAllowedServices  string
	IngestExcludedServices string
}

func Load() *Config {
	envFile := ".env"

	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		if err := godotenv.Load(envFile); err != nil {
			log.Println("⚠️  Failed to load .env file, using system environment variables or defaults")
		} else {
			log.Println("✅ Loaded configuration from .env")
		}
	} else {
		log.Println("⚠️  No .env file found, using system environment variables or defaults")
	}

	return &Config{
		Env:               getEnv("APP_ENV", "development"),
		LogLevel:          getEnv("LOG_LEVEL", "INFO"),
		HTTPPort:          getEnv("HTTP_PORT", "8080"),
		GRPCPort:          getEnv("GRPC_PORT", "4317"),
		DBDriver:          getEnv("DB_DRIVER", "sqlite"),
		DBDSN:             getEnv("DB_DSN", ""),
		DLQPath:           getEnv("DLQ_PATH", "./data/dlq"),
		DLQReplayInterval: getEnv("DLQ_REPLAY_INTERVAL", "5m"),

		IngestMinSeverity:      getEnv("INGEST_MIN_SEVERITY", "INFO"),
		IngestAllowedServices:  getEnv("INGEST_ALLOWED_SERVICES", ""),
		IngestExcludedServices: getEnv("INGEST_EXCLUDED_SERVICES", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
