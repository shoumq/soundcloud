package service

import (
	"context"
	"errors"
	"testing"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
)

func TestAuthServiceRegisterAndLogin(t *testing.T) {
	t.Parallel()

	auth := NewAuthService(newTestUserRepository(), "test-secret")
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

	auth := NewAuthService(newTestUserRepository(), "test-secret")
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

	auth := NewAuthService(newTestUserRepository(), "test-secret")
	ctx := context.Background()

	if _, err := auth.Register(ctx, "demo@example.com", "demo", "password123"); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := auth.Login(ctx, "demo@example.com", "wrong-password")
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
