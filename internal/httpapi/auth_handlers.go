package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"soundcloud/internal/repository"
	"soundcloud/internal/service"
)

type authRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	result, err := h.auth.Register(r.Context(), req.Email, req.Username, req.Password)
	if errors.Is(err, repository.ErrConflict) {
		writeError(w, http.StatusConflict, "email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	result, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) telegramAuth(w http.ResponseWriter, r *http.Request) {
	var req service.TelegramAuthData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	result, err := h.auth.LoginTelegram(r.Context(), req)
	if errors.Is(err, service.ErrTelegramNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "telegram auth is not configured")
		return
	}
	if errors.Is(err, service.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid telegram auth data")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram auth failed")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
