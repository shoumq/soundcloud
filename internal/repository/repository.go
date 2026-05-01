package repository

import (
	"context"
	"errors"

	"soundcloud/internal/domain"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindByID(ctx context.Context, id string) (domain.User, error)
	FindByTelegramID(ctx context.Context, telegramID string) (domain.User, error)
}

type TrackRepository interface {
	Create(ctx context.Context, track domain.Track) error
	FindByID(ctx context.Context, id string) (domain.Track, error)
	List(ctx context.Context) ([]domain.Track, error)
	ListByAlbumID(ctx context.Context, albumID string) ([]domain.Track, error)
}

type AlbumRepository interface {
	Create(ctx context.Context, album domain.Album) error
	FindByID(ctx context.Context, id string) (domain.Album, error)
	List(ctx context.Context) ([]domain.Album, error)
}
