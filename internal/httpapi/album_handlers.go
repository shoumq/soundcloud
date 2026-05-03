package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"soundcloud/internal/repository"
)

type albumRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type importSoundCloudAlbumRequest struct {
	URL string `json:"url"`
}

func (h *Handler) createAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req albumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	album, err := h.albums.Create(r.Context(), userID, req.Title, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, album)
}

func (h *Handler) importSoundCloudAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req importSoundCloudAlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	album, err := h.albums.ImportSoundCloud(r.Context(), userID, req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, album)
}

func (h *Handler) listAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.albums.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list albums")
		return
	}

	writeJSON(w, http.StatusOK, albums)
}

func (h *Handler) getAlbum(w http.ResponseWriter, r *http.Request) {
	album, err := h.albums.Find(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "album not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get album")
		return
	}

	writeJSON(w, http.StatusOK, album)
}

func (h *Handler) listAlbumTracks(w http.ResponseWriter, r *http.Request) {
	tracks, err := h.albums.Tracks(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "album not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list album tracks")
		return
	}

	writeJSON(w, http.StatusOK, tracks)
}
