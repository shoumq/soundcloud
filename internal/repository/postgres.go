package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"soundcloud/internal/domain"
)

type Postgres struct {
	db *pgxpool.Pool
}

func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db}
}

func (r *Postgres) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	username TEXT NOT NULL,
	telegram_id TEXT NOT NULL DEFAULT '',
	password_hash TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS users_telegram_id_idx ON users(telegram_id) WHERE telegram_id <> '';

CREATE TABLE IF NOT EXISTS tracks (
	id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	album_id TEXT,
	title TEXT NOT NULL,
	artist TEXT NOT NULL DEFAULT '',
	filename TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size BIGINT NOT NULL,
	storage_key TEXT NOT NULL,
	cover_filename TEXT NOT NULL DEFAULT '',
	cover_content_type TEXT NOT NULL DEFAULT '',
	cover_size BIGINT NOT NULL DEFAULT 0,
	cover_storage_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS albums (
	id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE tracks ADD COLUMN IF NOT EXISTS album_id TEXT;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS cover_filename TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS cover_content_type TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS cover_size BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS cover_storage_key TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'tracks_album_id_fkey'
	) THEN
		ALTER TABLE tracks
		ADD CONSTRAINT tracks_album_id_fkey
		FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE SET NULL;
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS tracks_created_at_idx ON tracks(created_at DESC);
CREATE INDEX IF NOT EXISTS tracks_owner_id_idx ON tracks(owner_id);
CREATE INDEX IF NOT EXISTS tracks_album_id_idx ON tracks(album_id);
CREATE INDEX IF NOT EXISTS albums_created_at_idx ON albums(created_at DESC);
CREATE INDEX IF NOT EXISTS albums_owner_id_idx ON albums(owner_id);
`)
	return err
}

func (r *Postgres) Create(ctx context.Context, user domain.User) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO users (id, email, username, telegram_id, password_hash, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
`, user.ID, user.Email, user.Username, user.TelegramID, user.PasswordHash, user.CreatedAt)
	return mapPostgresError(err)
}

func (r *Postgres) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, email, username, telegram_id, password_hash, created_at
FROM users
WHERE email = $1
`, email)
	return scanUser(row)
}

func (r *Postgres) FindByID(ctx context.Context, id string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, email, username, telegram_id, password_hash, created_at
FROM users
WHERE id = $1
`, id)
	return scanUser(row)
}

func (r *Postgres) FindByTelegramID(ctx context.Context, telegramID string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, email, username, telegram_id, password_hash, created_at
FROM users
WHERE telegram_id = $1
`, telegramID)
	return scanUser(row)
}

func (r *Postgres) CreateTrack(ctx context.Context, track domain.Track) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO tracks (
	id, owner_id, album_id, title, artist, filename, content_type, size, storage_key,
	cover_filename, cover_content_type, cover_size, cover_storage_key, created_at
)
VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
`, track.ID, track.OwnerID, track.AlbumID, track.Title, track.Artist, track.Filename, track.ContentType, track.Size, track.StorageKey, track.CoverFilename, track.CoverContentType, track.CoverSize, track.CoverStorageKey, track.CreatedAt)
	return mapPostgresError(err)
}

func (r *Postgres) FindTrackByID(ctx context.Context, id string) (domain.Track, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, owner_id, COALESCE(album_id, ''), title, artist, filename, content_type, size, storage_key,
	cover_filename, cover_content_type, cover_size, cover_storage_key, created_at
FROM tracks
WHERE id = $1
`, id)
	return scanTrack(row)
}

func (r *Postgres) ListTracks(ctx context.Context) ([]domain.Track, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, owner_id, COALESCE(album_id, ''), title, artist, filename, content_type, size, storage_key,
	cover_filename, cover_content_type, cover_size, cover_storage_key, created_at
FROM tracks
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]domain.Track, 0)
	for rows.Next() {
		track, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tracks, nil
}

func (r *Postgres) ListTracksByAlbumID(ctx context.Context, albumID string) ([]domain.Track, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, owner_id, COALESCE(album_id, ''), title, artist, filename, content_type, size, storage_key,
	cover_filename, cover_content_type, cover_size, cover_storage_key, created_at
FROM tracks
WHERE album_id = $1
ORDER BY created_at DESC
`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]domain.Track, 0)
	for rows.Next() {
		track, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tracks, nil
}

func (r *Postgres) CreateAlbum(ctx context.Context, album domain.Album) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO albums (id, owner_id, title, description, created_at)
VALUES ($1, $2, $3, $4, $5)
`, album.ID, album.OwnerID, album.Title, album.Description, album.CreatedAt)
	return mapPostgresError(err)
}

func (r *Postgres) FindAlbumByID(ctx context.Context, id string) (domain.Album, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, owner_id, title, description, created_at
FROM albums
WHERE id = $1
`, id)
	return scanAlbum(row)
}

func (r *Postgres) ListAlbums(ctx context.Context) ([]domain.Album, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, owner_id, title, description, created_at
FROM albums
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	albums := make([]domain.Album, 0)
	for rows.Next() {
		album, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, album)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return albums, nil
}

func (r *Postgres) CreateTrackRepository() TrackRepository {
	return postgresTrackRepository{db: r}
}

type postgresTrackRepository struct {
	db *Postgres
}

func (r postgresTrackRepository) Create(ctx context.Context, track domain.Track) error {
	return r.db.CreateTrack(ctx, track)
}

func (r postgresTrackRepository) FindByID(ctx context.Context, id string) (domain.Track, error) {
	return r.db.FindTrackByID(ctx, id)
}

func (r postgresTrackRepository) List(ctx context.Context) ([]domain.Track, error) {
	return r.db.ListTracks(ctx)
}

func (r postgresTrackRepository) ListByAlbumID(ctx context.Context, albumID string) ([]domain.Track, error) {
	return r.db.ListTracksByAlbumID(ctx, albumID)
}

func (r *Postgres) CreateAlbumRepository() AlbumRepository {
	return postgresAlbumRepository{db: r}
}

type postgresAlbumRepository struct {
	db *Postgres
}

func (r postgresAlbumRepository) Create(ctx context.Context, album domain.Album) error {
	return r.db.CreateAlbum(ctx, album)
}

func (r postgresAlbumRepository) FindByID(ctx context.Context, id string) (domain.Album, error) {
	return r.db.FindAlbumByID(ctx, id)
}

func (r postgresAlbumRepository) List(ctx context.Context) ([]domain.Album, error) {
	return r.db.ListAlbums(ctx)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (domain.User, error) {
	var user domain.User
	err := row.Scan(&user.ID, &user.Email, &user.Username, &user.TelegramID, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	return user, err
}

func scanTrack(row scanner) (domain.Track, error) {
	var track domain.Track
	err := row.Scan(
		&track.ID,
		&track.OwnerID,
		&track.AlbumID,
		&track.Title,
		&track.Artist,
		&track.Filename,
		&track.ContentType,
		&track.Size,
		&track.StorageKey,
		&track.CoverFilename,
		&track.CoverContentType,
		&track.CoverSize,
		&track.CoverStorageKey,
		&track.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Track{}, ErrNotFound
	}
	return track, err
}

func scanAlbum(row scanner) (domain.Album, error) {
	var album domain.Album
	err := row.Scan(&album.ID, &album.OwnerID, &album.Title, &album.Description, &album.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Album{}, ErrNotFound
	}
	return album, err
}

func mapPostgresError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
