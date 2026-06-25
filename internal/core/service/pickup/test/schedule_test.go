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
	"wst-backend/internal/core/port/in"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/pickup"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
)

func TestPickupService_Schedule(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	pickupDate := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	id := uuid.New()
	scheduled := domain.Pickup{ID: id, Type: domain.PickupOrganic, Status: domain.PickupScheduled, PickupDate: &pickupDate, CreatedAt: now, UpdatedAt: now}
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	tests := []struct {
		name    string
		setup   func(*repomock.PickupRepository)
		wantErr error
	}{
		{
			name: "happy",
			setup: func(p *repomock.PickupRepository) {
				p.On("Schedule", mock.Anything, id, pickupDate, now).Return(scheduled, true, nil).Once()
			},
		},
		{
			name: "repo infra error propagates",
			setup: func(p *repomock.PickupRepository) {
				p.On("Schedule", mock.Anything, id, pickupDate, now).Return(domain.Pickup{}, false, infraErr).Once()
			},
			wantErr: infraErr,
		},
		{
			name: "not pending",
			setup: func(p *repomock.PickupRepository) {
				p.On("Schedule", mock.Anything, id, pickupDate, now).Return(domain.Pickup{}, false, nil).Once()
				p.On("FindByID", mock.Anything, id).Return(domain.Pickup{ID: id, Status: domain.PickupScheduled, Type: domain.PickupOrganic}, nil).Once()
			},
			wantErr: domain.ErrPickupNotPending,
		},
		{
			name: "electronic without safety check",
			setup: func(p *repomock.PickupRepository) {
				p.On("Schedule", mock.Anything, id, pickupDate, now).Return(domain.Pickup{}, false, nil).Once()
				p.On("FindByID", mock.Anything, id).Return(domain.Pickup{ID: id, Status: domain.PickupPending, Type: domain.PickupElectronic, SafetyCheck: false}, nil).Once()
			},
			wantErr: domain.ErrSafetyCheckRequired,
		},
		{
			name: "not found",
			setup: func(p *repomock.PickupRepository) {
				p.On("Schedule", mock.Anything, id, pickupDate, now).Return(domain.Pickup{}, false, nil).Once()
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

			got, err := svc.Schedule(context.Background(), id, in.SchedulePickupCommand{PickupDate: pickupDate})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, domain.PickupScheduled, got.Status)
			}
			pickups.AssertExpectations(t)
		})
	}
}

func TestPickupService_Schedule_LostRaceAfterGuardPasses(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	pickupDate := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	id := uuid.New()
	pickups := new(repomock.PickupRepository)
	payments := new(repomock.PaymentRepository)
	svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)

	pickups.On("Schedule", mock.Anything, id, pickupDate, now).Return(domain.Pickup{}, false, nil).Once()
	pickups.On("FindByID", mock.Anything, id).Return(domain.Pickup{ID: id, Status: domain.PickupPending, Type: domain.PickupOrganic}, nil).Once()

	_, err := svc.Schedule(context.Background(), id, in.SchedulePickupCommand{PickupDate: pickupDate})

	require.ErrorIs(t, err, domain.ErrPickupNotPending)
	pickups.AssertExpectations(t)
}
