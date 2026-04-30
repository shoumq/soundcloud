package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"soundcloud/internal/service"
)

type RouterConfig struct {
	Auth      *service.AuthService
	Tracks    *service.TrackService
	JWTSecret string
	Logger    *slog.Logger
}

type Handler struct {
	auth   *service.AuthService
	tracks *service.TrackService
}

func NewRouter(cfg RouterConfig) http.Handler {
	h := &Handler{auth: cfg.Auth, tracks: cfg.Tracks}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
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

		r.Get("/tracks", h.listTracks)
		r.Get("/tracks/{id}", h.getTrack)
		r.Get("/tracks/{id}/stream", h.streamTrack)

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware(cfg.JWTSecret))
			r.Post("/tracks", h.uploadTrack)
		})
	})

	return r
}
