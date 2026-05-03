package httpapi

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"soundcloud/internal/repository"
)

type importSoundCloudTrackRequest struct {
	URL     string `json:"url"`
	AlbumID string `json:"album_id"`
}

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

	var coverHeader = (*multipart.FileHeader)(nil)
	var cover multipart.File
	cover, coverHeader, err = r.FormFile("cover")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		writeError(w, http.StatusBadRequest, "invalid cover file")
		return
	}
	if cover != nil {
		defer cover.Close()
	}

	track, err := h.tracks.Upload(r.Context(), userID, r.FormValue("title"), r.FormValue("album_id"), file, header, cover, coverHeader)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, track)
}

func (h *Handler) importSoundCloudTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req importSoundCloudTrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	track, err := h.tracks.ImportSoundCloud(r.Context(), userID, req.URL, req.AlbumID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, track)
}

func (h *Handler) listTracks(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := userIDFromContext(r.Context())
	tracks, err := h.tracks.ListForViewer(r.Context(), viewerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tracks")
		return
	}

	writeJSON(w, http.StatusOK, tracks)
}

func (h *Handler) getTrack(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := userIDFromContext(r.Context())
	track, err := h.tracks.FindForViewer(r.Context(), viewerID, chi.URLParam(r, "id"))
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

func (h *Handler) likeTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	track, err := h.tracks.Like(r.Context(), userID, chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, track)
}

func (h *Handler) unlikeTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	track, err := h.tracks.Unlike(r.Context(), userID, chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, track)
}

func (h *Handler) streamTrack(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	foundTrack, err := h.tracks.Find(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get track")
		return
	}
	if foundTrack.StorageKey == "" {
		writeError(w, http.StatusNotFound, "audio file not found")
		return
	}

	track, reader, err := h.tracks.Open(r.Context(), id)
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

func (h *Handler) streamTrackCover(w http.ResponseWriter, r *http.Request) {
	track, reader, err := h.tracks.OpenCover(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "cover not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open cover")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", track.CoverContentType)
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(track.CoverFilename)+`"`)
	http.ServeContent(w, r, track.CoverFilename, track.CreatedAt, reader)
}
