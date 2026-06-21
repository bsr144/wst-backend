package out

import (
	"context"
	"io"
)

type FileStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
}
