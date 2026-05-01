package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"soundcloud/internal/config"
	"soundcloud/internal/httpapi"
	"soundcloud/internal/repository"
	"soundcloud/internal/service"
	"soundcloud/internal/storage"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		logger.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	pg := repository.NewPostgres(db)
	if err := pg.Migrate(ctx); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	fileStorage, err := newAudioStorage(ctx, cfg)
	if err != nil {
		logger.Error("storage init failed", "error", err)
		os.Exit(1)
	}

	authService := service.NewAuthService(pg, cfg.JWTSecret, cfg.TelegramBotToken)
	trackRepository := pg.CreateTrackRepository()
	albumRepository := pg.CreateAlbumRepository()
	trackService := service.NewTrackService(trackRepository, pg, albumRepository, fileStorage)
	albumService := service.NewAlbumService(albumRepository, trackRepository)

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Auth:      authService,
		Tracks:    trackService,
		Albums:    albumService,
		JWTSecret: cfg.JWTSecret,
		Logger:    logger,
	})

	logger.Info("api listening", "addr", cfg.HTTPAddr, "storage_driver", cfg.StorageDriver)
	if err := http.ListenAndServe(cfg.HTTPAddr, router); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newAudioStorage(ctx context.Context, cfg config.Config) (storage.AudioStorage, error) {
	switch strings.ToLower(cfg.StorageDriver) {
	case "local":
		return storage.NewLocal(cfg.StorageDir), nil
	case "s3", "minio":
		return storage.NewS3(ctx, storage.S3Config{
			Endpoint:  cfg.S3.Endpoint,
			AccessKey: cfg.S3.AccessKey,
			SecretKey: cfg.S3.SecretKey,
			Bucket:    cfg.S3.Bucket,
			Region:    cfg.S3.Region,
			UseSSL:    cfg.S3.UseSSL,
		})
	default:
		return nil, config.ErrUnsupportedStorageDriver(cfg.StorageDriver)
	}
}
