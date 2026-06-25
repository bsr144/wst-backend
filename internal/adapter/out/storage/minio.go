package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"wst-backend/internal/config"
	"wst-backend/internal/core/port/out"
)

type MinIO struct {
	client   *minio.Client
	bucket   string
	endpoint string
	useSSL   bool
}

func NewMinIO(ctx context.Context, cfg config.MinIO) (*MinIO, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	if err := client.SetBucketPolicy(ctx, cfg.Bucket, publicReadPolicy(cfg.Bucket)); err != nil {
		return nil, err
	}

	return &MinIO{client: client, bucket: cfg.Bucket, endpoint: cfg.Endpoint, useSSL: cfg.UseSSL}, nil
}

func publicReadPolicy(bucket string) string {
	return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
}

func (m *MinIO) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := m.client.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}
	scheme := "http"
	if m.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, m.endpoint, m.bucket, key), nil
}

func (m *MinIO) Ping(ctx context.Context) error {
	_, err := m.client.BucketExists(ctx, m.bucket)
	return err
}

var _ out.FileStorage = (*MinIO)(nil)
