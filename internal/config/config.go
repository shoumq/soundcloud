package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr         string
	JWTSecret        string
	TelegramBotToken string
	AllowedOrigins   []string
	DatabaseURL      string
	StorageDriver    string
	StorageDir       string
	YTDLPBinary      string
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
		AllowedOrigins:   envList("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
		DatabaseURL:      env("DATABASE_URL", "postgres://soundcloud:soundcloud@localhost:5432/soundcloud?sslmode=disable"),
		StorageDriver:    env("STORAGE_DRIVER", "local"),
		StorageDir:       env("STORAGE_DIR", "var/tracks"),
		YTDLPBinary:      env("YT_DLP_BINARY", "yt-dlp"),
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

func envList(key, fallback string) []string {
	value := env(key, fallback)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	return values
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
