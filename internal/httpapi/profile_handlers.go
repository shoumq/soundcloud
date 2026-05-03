package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"soundcloud/internal/repository"
)

type updateProfileRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Bio      string `json:"bio"`
}

type updatePrivacyRequest struct {
	IsPrivate bool `json:"is_private"`
	ShowEmail bool `json:"show_email"`
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	profile, err := h.profiles.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) getUserProfile(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := userIDFromContext(r.Context())
	profile, err := h.profiles.Get(r.Context(), viewerID, chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	user, err := h.profiles.UpdateProfile(r.Context(), userID, req.Email, req.Username, req.Bio)
	if errors.Is(err, repository.ErrConflict) {
		writeError(w, http.StatusConflict, "email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) updatePrivacy(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req updatePrivacyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	user, err := h.profiles.UpdatePrivacy(r.Context(), userID, req.IsPrivate, req.ShowEmail)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	user, err := h.profiles.UpdateAvatar(r.Context(), userID, file, header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) streamUserAvatar(w http.ResponseWriter, r *http.Request) {
	user, reader, err := h.profiles.OpenAvatar(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open avatar")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", user.AvatarContentType)
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(user.AvatarFilename)+`"`)
	http.ServeContent(w, r, user.AvatarFilename, user.CreatedAt, reader)
}

func (h *Handler) followUser(w http.ResponseWriter, r *http.Request) {
	followerID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := h.profiles.Follow(r.Context(), followerID, chi.URLParam(r, "id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) unfollowUser(w http.ResponseWriter, r *http.Request) {
	followerID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.profiles.Unfollow(r.Context(), followerID, chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
