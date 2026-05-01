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
	users   repository.UserRepository
	albums  repository.AlbumRepository
	storage storage.AudioStorage
}

func NewTrackService(tracks repository.TrackRepository, users repository.UserRepository, albums repository.AlbumRepository, storage storage.AudioStorage) *TrackService {
	return &TrackService{tracks: tracks, users: users, albums: albums, storage: storage}
}

func (s *TrackService) Upload(ctx context.Context, ownerID, title, albumID string, file multipart.File, header *multipart.FileHeader, cover multipart.File, coverHeader *multipart.FileHeader) (domain.Track, error) {
	title = strings.TrimSpace(title)
	albumID = strings.TrimSpace(albumID)
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

	user, err := s.users.FindByID(ctx, ownerID)
	if err != nil {
		return domain.Track{}, err
	}

	if albumID != "" {
		album, err := s.albums.FindByID(ctx, albumID)
		if err != nil {
			return domain.Track{}, err
		}
		if album.OwnerID != ownerID {
			return domain.Track{}, errors.New("album does not belong to user")
		}
	}

	id := newID()
	storageKey, err := s.storage.Save(ctx, id, header.Filename, file)
	if err != nil {
		return domain.Track{}, err
	}

	var coverFilename, coverContentType, coverStorageKey string
	var coverSize int64
	if coverHeader != nil {
		if coverHeader.Size == 0 {
			return domain.Track{}, errors.New("cover file is empty")
		}
		if coverHeader.Size > 10<<20 {
			return domain.Track{}, errors.New("cover file is too large, max is 10MB")
		}

		var ok bool
		coverContentType, ok = imageContentType(coverHeader.Header.Get("Content-Type"), coverHeader.Filename)
		if !ok {
			return domain.Track{}, errors.New("only jpeg, png, webp or gif cover images are allowed")
		}

		coverStorageKey, err = s.storage.Save(ctx, id+"-cover", coverHeader.Filename, cover)
		if err != nil {
			return domain.Track{}, err
		}
		coverFilename = filepath.Base(coverHeader.Filename)
		coverSize = coverHeader.Size
	}

	track := domain.Track{
		ID:               id,
		OwnerID:          ownerID,
		AlbumID:          albumID,
		Title:            title,
		Artist:           user.Username,
		Filename:         filepath.Base(header.Filename),
		ContentType:      contentType,
		Size:             header.Size,
		StorageKey:       storageKey,
		CoverFilename:    coverFilename,
		CoverContentType: coverContentType,
		CoverSize:        coverSize,
		CoverStorageKey:  coverStorageKey,
		CreatedAt:        time.Now().UTC(),
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

func (s *TrackService) OpenCover(ctx context.Context, id string) (domain.Track, io.ReadSeekCloser, error) {
	track, err := s.tracks.FindByID(ctx, id)
	if err != nil {
		return domain.Track{}, nil, err
	}
	if track.CoverStorageKey == "" {
		return domain.Track{}, nil, repository.ErrNotFound
	}

	reader, err := s.storage.Open(ctx, track.CoverStorageKey)
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

func imageContentType(contentType, filename string) (string, bool) {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return contentType, true
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".png":
		return "image/png", true
	case ".webp":
		return "image/webp", true
	case ".gif":
		return "image/gif", true
	default:
		return "", false
	}
}
