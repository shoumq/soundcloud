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
	password_hash TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS tracks (
	id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	artist TEXT NOT NULL DEFAULT '',
	filename TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size BIGINT NOT NULL,
	storage_key TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS tracks_created_at_idx ON tracks(created_at DESC);
CREATE INDEX IF NOT EXISTS tracks_owner_id_idx ON tracks(owner_id);
`)
	return err
}

func (r *Postgres) Create(ctx context.Context, user domain.User) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO users (id, email, username, password_hash, created_at)
VALUES ($1, $2, $3, $4, $5)
`, user.ID, user.Email, user.Username, user.PasswordHash, user.CreatedAt)
	return mapPostgresError(err)
}

func (r *Postgres) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, email, username, password_hash, created_at
FROM users
WHERE email = $1
`, email)
	return scanUser(row)
}

func (r *Postgres) FindByID(ctx context.Context, id string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, email, username, password_hash, created_at
FROM users
WHERE id = $1
`, id)
	return scanUser(row)
}

func (r *Postgres) CreateTrack(ctx context.Context, track domain.Track) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO tracks (id, owner_id, title, artist, filename, content_type, size, storage_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, track.ID, track.OwnerID, track.Title, track.Artist, track.Filename, track.ContentType, track.Size, track.StorageKey, track.CreatedAt)
	return mapPostgresError(err)
}

func (r *Postgres) FindTrackByID(ctx context.Context, id string) (domain.Track, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, owner_id, title, artist, filename, content_type, size, storage_key, created_at
FROM tracks
WHERE id = $1
`, id)
	return scanTrack(row)
}

func (r *Postgres) ListTracks(ctx context.Context) ([]domain.Track, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, owner_id, title, artist, filename, content_type, size, storage_key, created_at
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

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (domain.User, error) {
	var user domain.User
	err := row.Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt)
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
		&track.Title,
		&track.Artist,
		&track.Filename,
		&track.ContentType,
		&track.Size,
		&track.StorageKey,
		&track.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Track{}, ErrNotFound
	}
	return track, err
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
