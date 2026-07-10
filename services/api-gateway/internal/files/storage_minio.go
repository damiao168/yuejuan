package files

import (
	"context"
	"io"

	"edugrade-enterprise/services/api-gateway/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOObjectStorage struct {
	client *minio.Client
}

func NewMinIOObjectStorage(cfg config.MinIOConfig) (*MinIOObjectStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinIOObjectStorage{client: client}, nil
}

func (s *MinIOObjectStorage) Put(ctx context.Context, bucket string, key string, body io.Reader, size int64, contentType string) error {
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	_, err = s.client.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *MinIOObjectStorage) Get(ctx context.Context, bucket string, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (s *MinIOObjectStorage) Remove(ctx context.Context, bucket string, key string) error {
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}
