package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr         string
	JWTSecret        string
	TelegramBotToken string
	DatabaseURL      string
	StorageDriver    string
	StorageDir       string
	S3               S3Config
}

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

func Load() Config {
	return Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		JWTSecret:        env("JWT_SECRET", "dev-secret-change-me"),
		TelegramBotToken: env("TELEGRAM_BOT_TOKEN", ""),
		DatabaseURL:      env("DATABASE_URL", "postgres://soundcloud:soundcloud@localhost:5432/soundcloud?sslmode=disable"),
		StorageDriver:    env("STORAGE_DRIVER", "local"),
		StorageDir:       env("STORAGE_DIR", "var/tracks"),
		S3: S3Config{
			Endpoint:  env("S3_ENDPOINT", "localhost:9000"),
			AccessKey: env("S3_ACCESS_KEY", "minioadmin"),
			SecretKey: env("S3_SECRET_KEY", "minioadmin"),
			Bucket:    env("S3_BUCKET", "tracks"),
			Region:    env("S3_REGION", "us-east-1"),
			UseSSL:    envBool("S3_USE_SSL", false),
		},
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func ErrUnsupportedStorageDriver(driver string) error {
	return fmt.Errorf("unsupported storage driver %q", driver)
}
