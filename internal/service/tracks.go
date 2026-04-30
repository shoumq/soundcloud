package service

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
	"soundcloud/internal/storage"
)

type TrackService struct {
	tracks  repository.TrackRepository
	storage storage.AudioStorage
}

func NewTrackService(tracks repository.TrackRepository, storage storage.AudioStorage) *TrackService {
	return &TrackService{tracks: tracks, storage: storage}
}

func (s *TrackService) Upload(ctx context.Context, ownerID, title, artist string, file multipart.File, header *multipart.FileHeader) (domain.Track, error) {
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	if title == "" {
		return domain.Track{}, errors.New("title is required")
	}
	if header == nil || header.Size == 0 {
		return domain.Track{}, errors.New("audio file is required")
	}
	if header.Size > 100<<20 {
		return domain.Track{}, errors.New("file is too large, max is 100MB")
	}

	contentType, ok := audioContentType(header.Header.Get("Content-Type"), header.Filename)
	if !ok {
		return domain.Track{}, errors.New("only mp3, wav, flac, m4a or ogg audio is allowed")
	}

	id := newID()
	storageKey, err := s.storage.Save(ctx, id, header.Filename, file)
	if err != nil {
		return domain.Track{}, err
	}

	track := domain.Track{
		ID:          id,
		OwnerID:     ownerID,
		Title:       title,
		Artist:      artist,
		Filename:    filepath.Base(header.Filename),
		ContentType: contentType,
		Size:        header.Size,
		StorageKey:  storageKey,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.tracks.Create(ctx, track); err != nil {
		return domain.Track{}, err
	}

	return track, nil
}

func (s *TrackService) List(ctx context.Context) ([]domain.Track, error) {
	return s.tracks.List(ctx)
}

func (s *TrackService) Find(ctx context.Context, id string) (domain.Track, error) {
	return s.tracks.FindByID(ctx, id)
}

func (s *TrackService) Open(ctx context.Context, id string) (domain.Track, io.ReadSeekCloser, error) {
	track, err := s.tracks.FindByID(ctx, id)
	if err != nil {
		return domain.Track{}, nil, err
	}

	reader, err := s.storage.Open(ctx, track.StorageKey)
	if err != nil {
		return domain.Track{}, nil, err
	}

	return track, reader, nil
}

func audioContentType(contentType, filename string) (string, bool) {
	switch strings.ToLower(contentType) {
	case "audio/mpeg", "audio/mp3", "audio/wav", "audio/x-wav", "audio/flac", "audio/mp4", "audio/ogg":
		return contentType, true
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mpeg", true
	case ".wav":
		return "audio/wav", true
	case ".flac":
		return "audio/flac", true
	case ".m4a":
		return "audio/mp4", true
	case ".ogg":
		return "audio/ogg", true
	default:
		return "", false
	}
}
