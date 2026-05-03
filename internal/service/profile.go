package service

import (
	"context"
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
	"soundcloud/internal/storage"
)

type ProfileService struct {
	users   repository.UserRepository
	tracks  repository.TrackRepository
	storage storage.AudioStorage
}

type UserProfile struct {
	User           domain.User    `json:"user"`
	Tracks         []domain.Track `json:"tracks"`
	Following      []domain.User  `json:"following"`
	FollowersCount int            `json:"followers_count"`
	FollowingCount int            `json:"following_count"`
	IsFollowing    bool           `json:"is_following"`
	CanViewTracks  bool           `json:"can_view_tracks"`
	IsOwner        bool           `json:"is_owner"`
}

func NewProfileService(users repository.UserRepository, tracks repository.TrackRepository, fileStorage storage.AudioStorage) *ProfileService {
	return &ProfileService{users: users, tracks: tracks, storage: fileStorage}
}

func (s *ProfileService) Me(ctx context.Context, userID string) (UserProfile, error) {
	return s.profile(ctx, userID, userID)
}

func (s *ProfileService) Get(ctx context.Context, viewerID, targetUserID string) (UserProfile, error) {
	return s.profile(ctx, viewerID, targetUserID)
}

func (s *ProfileService) profile(ctx context.Context, viewerID, targetUserID string) (UserProfile, error) {
	user, err := s.users.FindByID(ctx, targetUserID)
	if err != nil {
		return UserProfile{}, err
	}

	isOwner := viewerID != "" && viewerID == targetUserID
	canViewTracks := isOwner || !user.IsPrivate

	followersCount, err := s.users.CountFollowers(ctx, targetUserID)
	if err != nil {
		return UserProfile{}, err
	}
	followingCount, err := s.users.CountFollowing(ctx, targetUserID)
	if err != nil {
		return UserProfile{}, err
	}

	following, err := s.users.ListFollowing(ctx, targetUserID)
	if err != nil {
		return UserProfile{}, err
	}
	following = sanitizeUsers(following, false)

	isFollowing := false
	if viewerID != "" && !isOwner {
		isFollowing, err = s.users.IsFollowing(ctx, viewerID, targetUserID)
		if err != nil {
			return UserProfile{}, err
		}
	}

	tracks := make([]domain.Track, 0)
	if canViewTracks {
		tracks, err = s.tracks.ListByOwnerID(ctx, targetUserID)
		if err != nil {
			return UserProfile{}, err
		}
		tracks, err = enrichTracksWithLikes(ctx, s.tracks, viewerID, tracks)
		if err != nil {
			return UserProfile{}, err
		}
	}

	return UserProfile{
		User:           sanitizeUser(user, isOwner),
		Tracks:         tracks,
		Following:      following,
		FollowersCount: followersCount,
		FollowingCount: followingCount,
		IsFollowing:    isFollowing,
		CanViewTracks:  canViewTracks,
		IsOwner:        isOwner,
	}, nil
}

func (s *ProfileService) UpdateProfile(ctx context.Context, userID, email, username, bio string) (domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}

	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)
	bio = strings.TrimSpace(bio)

	if email == "" || username == "" {
		return domain.User{}, errors.New("email and username are required")
	}

	user.Email = email
	user.Username = username
	user.Bio = bio

	if err := s.users.Update(ctx, user); err != nil {
		return domain.User{}, err
	}
	if err := s.tracks.UpdateArtistByOwnerID(ctx, userID, username); err != nil {
		return domain.User{}, err
	}

	return sanitizeUser(user, true), nil
}

func (s *ProfileService) UpdatePrivacy(ctx context.Context, userID string, isPrivate, showEmail bool) (domain.User, error) {
	if err := s.users.UpdatePrivacy(ctx, userID, isPrivate, showEmail); err != nil {
		return domain.User{}, err
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	return sanitizeUser(user, true), nil
}

func (s *ProfileService) UpdateAvatar(ctx context.Context, userID string, file multipart.File, header *multipart.FileHeader) (domain.User, error) {
	if header == nil || header.Size == 0 {
		return domain.User{}, errors.New("avatar file is required")
	}
	if header.Size > 10<<20 {
		return domain.User{}, errors.New("avatar file is too large, max is 10MB")
	}

	contentType, ok := profileImageContentType(header.Header.Get("Content-Type"), header.Filename)
	if !ok {
		return domain.User{}, errors.New("only jpeg, png, webp or gif avatar images are allowed")
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}

	storageKey, err := s.storage.Save(ctx, userID+"-avatar", header.Filename, file)
	if err != nil {
		return domain.User{}, err
	}

	user.AvatarFilename = filepath.Base(header.Filename)
	user.AvatarContentType = contentType
	user.AvatarStorageKey = storageKey

	if err := s.users.Update(ctx, user); err != nil {
		return domain.User{}, err
	}

	return sanitizeUser(user, true), nil
}

func (s *ProfileService) OpenAvatar(ctx context.Context, userID string) (domain.User, storage.ReadSeekCloser, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.User{}, nil, err
	}
	if user.AvatarStorageKey == "" {
		return domain.User{}, nil, repository.ErrNotFound
	}

	reader, err := s.storage.Open(ctx, user.AvatarStorageKey)
	if err != nil {
		return domain.User{}, nil, err
	}
	return user, reader, nil
}

func (s *ProfileService) Follow(ctx context.Context, followerID, followeeID string) error {
	if followerID == "" || followeeID == "" {
		return errors.New("user ids are required")
	}
	if followerID == followeeID {
		return errors.New("cannot follow yourself")
	}
	if _, err := s.users.FindByID(ctx, followeeID); err != nil {
		return err
	}
	return s.users.Follow(ctx, followerID, followeeID)
}

func (s *ProfileService) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if followerID == "" || followeeID == "" {
		return errors.New("user ids are required")
	}
	return s.users.Unfollow(ctx, followerID, followeeID)
}

func sanitizeUsers(users []domain.User, includeEmail bool) []domain.User {
	result := make([]domain.User, 0, len(users))
	for _, user := range users {
		result = append(result, sanitizeUser(user, includeEmail))
	}
	return result
}

func sanitizeUser(user domain.User, includeEmail bool) domain.User {
	user.PasswordHash = ""
	if !includeEmail && !user.ShowEmail {
		user.Email = ""
	}
	return user
}

func profileImageContentType(contentType, filename string) (string, bool) {
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
