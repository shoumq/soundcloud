package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"soundcloud/internal/service"
)

type RouterConfig struct {
	Auth           *service.AuthService
	Tracks         *service.TrackService
	Albums         *service.AlbumService
	JWTSecret      string
	AllowedOrigins []string
	Logger         *slog.Logger
}

type Handler struct {
	auth   *service.AuthService
	tracks *service.TrackService
	albums *service.AlbumService
}

func NewRouter(cfg RouterConfig) http.Handler {
	h := &Handler{auth: cfg.Auth, tracks: cfg.Tracks, albums: cfg.Albums}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.AllowedOrigins))
	if cfg.Logger != nil {
		r.Use(requestLogger(cfg.Logger))
	}

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/swagger", swaggerUI)
	r.Get("/swagger/openapi.yaml", openAPIYAML)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)
		r.Post("/auth/telegram", h.telegramAuth)

		r.Get("/tracks", h.listTracks)
		r.Get("/tracks/{id}", h.getTrack)
		r.Get("/tracks/{id}/stream", h.streamTrack)
		r.Get("/tracks/{id}/cover", h.streamTrackCover)
		r.Get("/albums", h.listAlbums)
		r.Get("/albums/{id}", h.getAlbum)
		r.Get("/albums/{id}/tracks", h.listAlbumTracks)

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware(cfg.JWTSecret))
			r.Post("/tracks", h.uploadTrack)
			r.Post("/albums", h.createAlbum)
		})
	})

	return r
}
