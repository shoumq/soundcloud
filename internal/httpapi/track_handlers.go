package httpapi

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"soundcloud/internal/repository"
)

func (h *Handler) uploadTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := r.ParseMultipartForm(110 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "audio file is required")
		return
	}
	defer file.Close()

	track, err := h.tracks.Upload(r.Context(), userID, r.FormValue("title"), r.FormValue("artist"), file, header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, track)
}

func (h *Handler) listTracks(w http.ResponseWriter, r *http.Request) {
	tracks, err := h.tracks.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tracks")
		return
	}

	writeJSON(w, http.StatusOK, tracks)
}

func (h *Handler) getTrack(w http.ResponseWriter, r *http.Request) {
	track, err := h.tracks.Find(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get track")
		return
	}

	writeJSON(w, http.StatusOK, track)
}

func (h *Handler) streamTrack(w http.ResponseWriter, r *http.Request) {
	track, reader, err := h.tracks.Open(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open track")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", track.ContentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(track.Filename)+`"`)
	http.ServeContent(w, r, track.Filename, track.CreatedAt, reader)
}
