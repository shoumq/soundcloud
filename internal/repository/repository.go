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
	Update(ctx context.Context, user domain.User) error
	UpdatePrivacy(ctx context.Context, userID string, isPrivate, showEmail bool) error
	ListFollowing(ctx context.Context, userID string) ([]domain.User, error)
	ListFollowers(ctx context.Context, userID string) ([]domain.User, error)
	Follow(ctx context.Context, followerID, followeeID string) error
	Unfollow(ctx context.Context, followerID, followeeID string) error
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	CountFollowers(ctx context.Context, userID string) (int, error)
	CountFollowing(ctx context.Context, userID string) (int, error)
}

type TrackRepository interface {
	Create(ctx context.Context, track domain.Track) error
	FindByID(ctx context.Context, id string) (domain.Track, error)
	List(ctx context.Context) ([]domain.Track, error)
	ListByAlbumID(ctx context.Context, albumID string) ([]domain.Track, error)
	ListByOwnerID(ctx context.Context, ownerID string) ([]domain.Track, error)
	UpdateArtistByOwnerID(ctx context.Context, ownerID, artist string) error
	Like(ctx context.Context, userID, trackID string) error
	Unlike(ctx context.Context, userID, trackID string) error
	CountLikesByTrackIDs(ctx context.Context, trackIDs []string) (map[string]int, error)
	ListLikedTrackIDs(ctx context.Context, userID string, trackIDs []string) (map[string]struct{}, error)
}

type AlbumRepository interface {
	Create(ctx context.Context, album domain.Album) error
	FindByID(ctx context.Context, id string) (domain.Album, error)
	List(ctx context.Context) ([]domain.Album, error)
}
