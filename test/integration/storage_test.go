//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"

	"wst-backend/adapter/out/storage"
	"wst-backend/config"
)

func TestMinIOStorage_PutRoundTrip(t *testing.T) {
	ctx := context.Background()

	container, err := tcminio.Run(ctx, "minio/minio:latest",
		tcminio.WithUsername("minioadmin"),
		tcminio.WithPassword("minioadmin"),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/minio/health/ready").
				WithPort("9000/tcp").
				WithStartupTimeout(60*time.Second)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	endpoint, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	store, err := storage.NewMinIO(ctx, config.MinIO{
		Endpoint:  endpoint,
		AccessKey: container.Username,
		SecretKey: container.Password,
		Bucket:    "proofs",
		UseSSL:    false,
	})
	require.NoError(t, err)

	content := []byte("\x89PNG\r\n\x1a\n integration proof bytes")
	key := "payments/abc/def.png"
	url, err := store.Put(ctx, key, bytes.NewReader(content), int64(len(content)), "image/png")
	require.NoError(t, err)
	assert.Equal(t, "http://"+endpoint+"/proofs/"+key, url)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(container.Username, container.Password, ""),
		Secure: false,
	})
	require.NoError(t, err)
	obj, err := client.GetObject(ctx, "proofs", key, minio.GetObjectOptions{})
	require.NoError(t, err)
	defer obj.Close()

	info, err := obj.Stat()
	require.NoError(t, err)
	assert.Equal(t, "image/png", info.ContentType)

	got, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}
