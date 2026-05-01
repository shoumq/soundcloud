package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrTelegramNotConfigured = errors.New("telegram auth is not configured")

type AuthService struct {
	users            repository.UserRepository
	jwtSecret        string
	telegramBotToken string
}

type AuthResult struct {
	User  domain.User `json:"user"`
	Token string      `json:"token"`
}

type TelegramAuthData struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

func NewAuthService(users repository.UserRepository, jwtSecret, telegramBotToken string) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret, telegramBotToken: strings.TrimSpace(telegramBotToken)}
}

func (s *AuthService) Register(ctx context.Context, email, username, password string) (AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if email == "" || username == "" || len(password) < 8 {
		return AuthResult{}, errors.New("email, username and password with at least 8 chars are required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}

	user := domain.User{
		ID:           newID(),
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return AuthResult{}, err
	}

	token, err := s.TokenFor(user)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{User: user, Token: token}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	user, err := s.users.FindByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	token, err := s.TokenFor(user)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{User: user, Token: token}, nil
}

func (s *AuthService) LoginTelegram(ctx context.Context, data TelegramAuthData) (AuthResult, error) {
	if err := s.validateTelegramAuthData(data); err != nil {
		return AuthResult{}, err
	}

	telegramID := strconv.FormatInt(data.ID, 10)
	user, err := s.users.FindByTelegramID(ctx, telegramID)
	if err == nil {
		token, err := s.TokenFor(user)
		if err != nil {
			return AuthResult{}, err
		}
		return AuthResult{User: user, Token: token}, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return AuthResult{}, err
	}

	user = domain.User{
		ID:           newID(),
		Email:        fmt.Sprintf("telegram_%s@telegram.local", telegramID),
		Username:     telegramUsername(data),
		TelegramID:   telegramID,
		PasswordHash: "",
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return AuthResult{}, err
	}

	token, err := s.TokenFor(user)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{User: user, Token: token}, nil
}

func (s *AuthService) TokenFor(user domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"email":    user.Email,
		"username": user.Username,
		"telegram": user.TelegramID,
		"exp":      time.Now().UTC().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().UTC().Unix(),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) validateTelegramAuthData(data TelegramAuthData) error {
	if s.telegramBotToken == "" {
		return ErrTelegramNotConfigured
	}
	if data.ID <= 0 || data.AuthDate <= 0 || strings.TrimSpace(data.Hash) == "" {
		return ErrInvalidCredentials
	}

	authTime := time.Unix(data.AuthDate, 0)
	now := time.Now()
	if now.Sub(authTime) > 24*time.Hour || authTime.After(now.Add(5*time.Minute)) {
		return ErrInvalidCredentials
	}

	values := map[string]string{
		"id":        strconv.FormatInt(data.ID, 10),
		"auth_date": strconv.FormatInt(data.AuthDate, 10),
	}
	if data.FirstName != "" {
		values["first_name"] = data.FirstName
	}
	if data.LastName != "" {
		values["last_name"] = data.LastName
	}
	if data.Username != "" {
		values["username"] = data.Username
	}
	if data.PhotoURL != "" {
		values["photo_url"] = data.PhotoURL
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}

	secret := sha256.Sum256([]byte(s.telegramBotToken))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(strings.Join(parts, "\n")))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(data.Hash))) {
		return ErrInvalidCredentials
	}

	return nil
}

func telegramUsername(data TelegramAuthData) string {
	username := strings.TrimSpace(data.Username)
	if username != "" {
		return username
	}

	name := strings.TrimSpace(strings.Join([]string{data.FirstName, data.LastName}, " "))
	if name != "" {
		return name
	}

	return "telegram_" + strconv.FormatInt(data.ID, 10)
}
