package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
)

type AlbumService struct {
	albums repository.AlbumRepository
	tracks repository.TrackRepository
}

func NewAlbumService(albums repository.AlbumRepository, tracks repository.TrackRepository) *AlbumService {
	return &AlbumService{albums: albums, tracks: tracks}
}

func (s *AlbumService) Create(ctx context.Context, ownerID, title, description string) (domain.Album, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return domain.Album{}, errors.New("title is required")
	}

	album := domain.Album{
		ID:          newID(),
		OwnerID:     ownerID,
		Title:       title,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.albums.Create(ctx, album); err != nil {
		return domain.Album{}, err
	}

	return album, nil
}

func (s *AlbumService) List(ctx context.Context) ([]domain.Album, error) {
	return s.albums.List(ctx)
}

func (s *AlbumService) Find(ctx context.Context, id string) (domain.Album, error) {
	return s.albums.FindByID(ctx, id)
}

func (s *AlbumService) Tracks(ctx context.Context, id string) ([]domain.Track, error) {
	if _, err := s.albums.FindByID(ctx, id); err != nil {
		return nil, err
	}
	return s.tracks.ListByAlbumID(ctx, id)
}
