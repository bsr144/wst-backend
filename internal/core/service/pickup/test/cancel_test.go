package pickup_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/pickup"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
)

func TestPickupService_Cancel(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	canceled := domain.Pickup{ID: id, Status: domain.PickupCanceled, CreatedAt: now, UpdatedAt: now}

	tests := []struct {
		name    string
		setup   func(*repomock.PickupRepository)
		wantErr error
	}{
		{
			name: "happy",
			setup: func(p *repomock.PickupRepository) {
				p.On("Cancel", mock.Anything, id, now).Return(canceled, true, nil).Once()
			},
		},
		{
			name: "not cancelable",
			setup: func(p *repomock.PickupRepository) {
				p.On("Cancel", mock.Anything, id, now).Return(domain.Pickup{}, false, nil).Once()
				p.On("FindByID", mock.Anything, id).Return(domain.Pickup{ID: id, Status: domain.PickupCompleted}, nil).Once()
			},
			wantErr: domain.ErrPickupNotCancelable,
		},
		{
			name: "not found",
			setup: func(p *repomock.PickupRepository) {
				p.On("Cancel", mock.Anything, id, now).Return(domain.Pickup{}, false, nil).Once()
				p.On("FindByID", mock.Anything, id).Return(domain.Pickup{}, domain.ErrPickupNotFound).Once()
			},
			wantErr: domain.ErrPickupNotFound,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pickups := new(repomock.PickupRepository)
			payments := new(repomock.PaymentRepository)
			svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
			tc.setup(pickups)

			got, err := svc.Cancel(context.Background(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, domain.PickupCanceled, got.Status)
			}
			pickups.AssertExpectations(t)
		})
	}
}

func TestPickupService_Cancel_RepoError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	pickups := new(repomock.PickupRepository)
	payments := new(repomock.PaymentRepository)
	svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)

	pickups.On("Cancel", mock.Anything, id, now).Return(domain.Pickup{}, false, infraErr).Once()

	_, err := svc.Cancel(context.Background(), id)

	require.ErrorIs(t, err, infraErr)
	pickups.AssertExpectations(t)
}
