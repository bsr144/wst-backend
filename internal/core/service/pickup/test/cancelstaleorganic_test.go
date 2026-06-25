package pickup_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/pickup"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
)

func TestPickupService_CancelStaleOrganic(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	cutoff := now.Add(-servicetest.OrganicTTL)
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	tests := []struct {
		name      string
		setup     func(*repomock.PickupRepository)
		wantCount int
		wantErr   error
	}{
		{
			name: "cancels stale organic pickups",
			setup: func(p *repomock.PickupRepository) {
				p.On("CancelStaleOrganic", mock.Anything, cutoff, now).Return(3, nil).Once()
			},
			wantCount: 3,
		},
		{
			name: "none stale returns zero",
			setup: func(p *repomock.PickupRepository) {
				p.On("CancelStaleOrganic", mock.Anything, cutoff, now).Return(0, nil).Once()
			},
			wantCount: 0,
		},
		{
			name: "repo infra error propagates",
			setup: func(p *repomock.PickupRepository) {
				p.On("CancelStaleOrganic", mock.Anything, cutoff, now).Return(0, infraErr).Once()
			},
			wantErr: infraErr,
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

			got, err := svc.CancelStaleOrganic(context.Background())

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantCount, got)
			}
			pickups.AssertExpectations(t)
		})
	}
}
