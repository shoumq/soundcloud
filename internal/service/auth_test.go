package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
)

func TestAuthServiceRegisterAndLogin(t *testing.T) {
	t.Parallel()

	auth := NewAuthService(newTestUserRepository(), "test-secret", "")
	ctx := context.Background()

	registered, err := auth.Register(ctx, "Demo@Example.com", "demo", "password123")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if registered.User.Email != "demo@example.com" {
		t.Fatalf("expected normalized email, got %q", registered.User.Email)
	}
	if registered.Token == "" {
		t.Fatal("expected register token")
	}

	loggedIn, err := auth.Login(ctx, "demo@example.com", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if loggedIn.User.ID != registered.User.ID {
		t.Fatalf("expected same user id, got %q", loggedIn.User.ID)
	}
}

func TestAuthServiceRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	auth := NewAuthService(newTestUserRepository(), "test-secret", "")
	ctx := context.Background()

	if _, err := auth.Register(ctx, "demo@example.com", "demo", "password123"); err != nil {
		t.Fatalf("register first user: %v", err)
	}

	_, err := auth.Register(ctx, "demo@example.com", "demo2", "password123")
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestAuthServiceRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	auth := NewAuthService(newTestUserRepository(), "test-secret", "")
	ctx := context.Background()

	if _, err := auth.Register(ctx, "demo@example.com", "demo", "password123"); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := auth.Login(ctx, "demo@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthServiceLoginTelegramRegistersAndLogsIn(t *testing.T) {
	t.Parallel()

	const botToken = "123456:test-token"
	auth := NewAuthService(newTestUserRepository(), "test-secret", botToken)
	ctx := context.Background()
	data := signedTelegramAuthData(botToken, map[string]string{
		"id":         "42",
		"first_name": "Demo",
		"username":   "telegram_demo",
		"auth_date":  strconv.FormatInt(time.Now().Unix(), 10),
	})

	registered, err := auth.LoginTelegram(ctx, data)
	if err != nil {
		t.Fatalf("telegram register: %v", err)
	}
	if registered.User.TelegramID != "42" {
		t.Fatalf("expected telegram id, got %q", registered.User.TelegramID)
	}
	if registered.User.Username != "telegram_demo" {
		t.Fatalf("expected telegram username, got %q", registered.User.Username)
	}
	if registered.Token == "" {
		t.Fatal("expected telegram token")
	}

	loggedIn, err := auth.LoginTelegram(ctx, data)
	if err != nil {
		t.Fatalf("telegram login: %v", err)
	}
	if loggedIn.User.ID != registered.User.ID {
		t.Fatalf("expected same user id, got %q", loggedIn.User.ID)
	}
}

func TestAuthServiceLoginTelegramRejectsInvalidHash(t *testing.T) {
	t.Parallel()

	auth := NewAuthService(newTestUserRepository(), "test-secret", "123456:test-token")
	_, err := auth.LoginTelegram(context.Background(), serviceTelegramData(42, time.Now().Unix(), "bad-hash"))
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

type testUserRepository struct {
	byID    map[string]domain.User
	byEmail map[string]string
}

func newTestUserRepository() *testUserRepository {
	return &testUserRepository{
		byID:    make(map[string]domain.User),
		byEmail: make(map[string]string),
	}
}

func (r *testUserRepository) Create(_ context.Context, user domain.User) error {
	if _, exists := r.byEmail[user.Email]; exists {
		return repository.ErrConflict
	}
	r.byID[user.ID] = user
	r.byEmail[user.Email] = user.ID
	return nil
}

func (r *testUserRepository) FindByEmail(_ context.Context, email string) (domain.User, error) {
	id, exists := r.byEmail[email]
	if !exists {
		return domain.User{}, repository.ErrNotFound
	}
	return r.byID[id], nil
}

func (r *testUserRepository) FindByID(_ context.Context, id string) (domain.User, error) {
	user, exists := r.byID[id]
	if !exists {
		return domain.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (r *testUserRepository) FindByTelegramID(_ context.Context, telegramID string) (domain.User, error) {
	for _, user := range r.byID {
		if user.TelegramID == telegramID {
			return user, nil
		}
	}
	return domain.User{}, repository.ErrNotFound
}

func signedTelegramAuthData(botToken string, values map[string]string) TelegramAuthData {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}

	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(strings.Join(parts, "\n")))

	id, _ := strconv.ParseInt(values["id"], 10, 64)
	authDate, _ := strconv.ParseInt(values["auth_date"], 10, 64)
	return TelegramAuthData{
		ID:        id,
		FirstName: values["first_name"],
		LastName:  values["last_name"],
		Username:  values["username"],
		PhotoURL:  values["photo_url"],
		AuthDate:  authDate,
		Hash:      hex.EncodeToString(mac.Sum(nil)),
	}
}

func serviceTelegramData(id int64, authDate int64, hash string) TelegramAuthData {
	return TelegramAuthData{
		ID:       id,
		AuthDate: authDate,
		Hash:     hash,
	}
}
