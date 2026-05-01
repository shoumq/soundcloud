package domain

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	TelegramID   string    `json:"telegram_id,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
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
	CreatedAt        time.Time `json:"created_at"`
}

type Album struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
