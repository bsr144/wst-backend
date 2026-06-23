package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"wst-backend/adapter/in/worker"
	"wst-backend/core/port/in"
)

type fakePickupService struct {
	in.PickupService
	count int
	err   error
	calls int
}

func (f *fakePickupService) CancelStaleOrganic(ctx context.Context) (int, error) {
	f.calls++
	return f.count, f.err
}

func TestAutoCancelOrganic_Job(t *testing.T) {
	t.Parallel()

	t.Run("runs the sweep and returns nil on success", func(t *testing.T) {
		t.Parallel()
		fake := &fakePickupService{count: 5}
		job := worker.AutoCancelOrganic(fake, time.Minute, zap.NewNop())

		assert.Equal(t, "autocancel_organic", job.Name)
		assert.Equal(t, time.Minute, job.Interval)
		require.NoError(t, job.Run(context.Background()))
		assert.Equal(t, 1, fake.calls)
	})

	t.Run("propagates the service error", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("sweep failed")
		fake := &fakePickupService{err: wantErr}
		job := worker.AutoCancelOrganic(fake, time.Minute, zap.NewNop())

		require.ErrorIs(t, job.Run(context.Background()), wantErr)
	})
}

func TestScheduler_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ran := make(chan struct{}, 1)
	job := worker.Job{
		Name:     "test",
		Interval: time.Millisecond,
		Run: func(ctx context.Context) error {
			select {
			case ran <- struct{}{}:
			default:
			}
			return nil
		},
	}
	scheduler := worker.New(zap.NewNop(), job)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- scheduler.Start(ctx) }()

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("job did not run before cancel")
	}

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop after context cancel")
	}
}
