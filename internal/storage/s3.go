package storage

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

type S3 struct {
	client *minio.Client
	bucket string
	region string
}

func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, err
		}
	}

	return &S3{
		client: client,
		bucket: cfg.Bucket,
		region: cfg.Region,
	}, nil
}

func (s *S3) Save(ctx context.Context, trackID, filename string, src io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	key := trackID + ext

	_, err := s.client.PutObject(ctx, s.bucket, key, src, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}

	return key, nil
}

func (s *S3) Open(ctx context.Context, key string) (ReadSeekCloser, error) {
	return s.client.GetObject(ctx, s.bucket, filepath.Base(key), minio.GetObjectOptions{})
}
