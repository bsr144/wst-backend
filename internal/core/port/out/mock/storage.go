package mock

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"

	"wst-backend/internal/core/port/out"
)

type FileStorage struct {
	mock.Mock
}

func (m *FileStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	args := m.Called(ctx, key, r, size, contentType)
	return args.String(0), args.Error(1)
}

var _ out.FileStorage = (*FileStorage)(nil)
