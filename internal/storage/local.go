package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type AudioStorage interface {
	Save(ctx context.Context, trackID, filename string, src io.Reader) (string, error)
	Open(ctx context.Context, key string) (ReadSeekCloser, error)
}

type Local struct {
	dir string
}

func NewLocal(dir string) *Local {
	return &Local{dir: dir}
}

func (s *Local) Save(_ context.Context, trackID, filename string, src io.Reader) (string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	key := trackID + ext
	path := filepath.Join(s.dir, key)

	dst, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return key, nil
}

func (s *Local) Open(_ context.Context, key string) (ReadSeekCloser, error) {
	return os.Open(filepath.Join(s.dir, filepath.Base(key)))
}
