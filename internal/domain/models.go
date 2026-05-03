package domain

import "time"

type User struct {
	ID                string    `json:"id"`
	Email             string    `json:"email,omitempty"`
	Username          string    `json:"username"`
	Bio               string    `json:"bio,omitempty"`
	TelegramID        string    `json:"telegram_id,omitempty"`
	AvatarFilename    string    `json:"avatar_filename,omitempty"`
	AvatarContentType string    `json:"avatar_content_type,omitempty"`
	AvatarStorageKey  string    `json:"-"`
	IsPrivate         bool      `json:"is_private"`
	ShowEmail         bool      `json:"show_email"`
	PasswordHash      string    `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
}

type Follow struct {
	FollowerID string    `json:"follower_id"`
	FolloweeID string    `json:"followee_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Track struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id"`
	AlbumID          string    `json:"album_id,omitempty"`
	Title            string    `json:"title"`
	Artist           string    `json:"artist"`
	Filename         string    `json:"filename"`
	ContentType      string    `json:"content_type"`
	Size             int64     `json:"size"`
	StorageKey       string    `json:"-"`
	CoverFilename    string    `json:"cover_filename,omitempty"`
	CoverContentType string    `json:"cover_content_type,omitempty"`
	CoverSize        int64     `json:"cover_size,omitempty"`
	CoverStorageKey  string    `json:"-"`
	ArtworkURL       string    `json:"artwork_url,omitempty"`
	SourceProvider   string    `json:"source_provider,omitempty"`
	SourceURL        string    `json:"source_url,omitempty"`
	EmbedHTML        string    `json:"embed_html,omitempty"`
	LikesCount       int       `json:"likes_count"`
	LikedByMe        bool      `json:"liked_by_me"`
	CreatedAt        time.Time `json:"created_at"`
}

type Album struct {
	ID             string    `json:"id"`
	OwnerID        string    `json:"owner_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	ArtworkURL     string    `json:"artwork_url,omitempty"`
	SourceProvider string    `json:"source_provider,omitempty"`
	SourceURL      string    `json:"source_url,omitempty"`
	EmbedHTML      string    `json:"embed_html,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
