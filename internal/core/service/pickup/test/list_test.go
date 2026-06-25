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
	"wst-backend/internal/pkg/pagination"
)

func TestPickupService_List(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	householdID := uuid.New()
	status := domain.PickupPending
	items := []domain.Pickup{
		{ID: uuid.New(), HouseholdID: householdID, Type: domain.PickupOrganic, Status: domain.PickupPending, CreatedAt: now, UpdatedAt: now},
	}
	pickups := new(repomock.PickupRepository)
	payments := new(repomock.PaymentRepository)
	svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)

	params := pagination.Params{Page: 2, PerPage: 10}
	pickups.On("List", mock.Anything, &status, &householdID, 10, 10).Return(items, nil).Once()
	pickups.On("Count", mock.Anything, &status, &householdID).Return(7, nil).Once()

	got, total, err := svc.List(context.Background(), in.PickupFilter{Status: &status, HouseholdID: &householdID}, params)

	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, 7, total)
	pickups.AssertExpectations(t)
}

func TestPickupService_List_Error(t *testing.T) {
	t.Parallel()

	wantErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	pickups := new(repomock.PickupRepository)
	payments := new(repomock.PaymentRepository)
	svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: time.Now()}, servicetest.Pricing, servicetest.OrganicTTL)

	pickups.On("List", mock.Anything, (*domain.PickupStatus)(nil), (*uuid.UUID)(nil), 20, 0).Return(nil, wantErr).Once()

	_, _, err := svc.List(context.Background(), in.PickupFilter{}, pagination.Params{Page: 1, PerPage: 20})

	require.ErrorIs(t, err, wantErr)
	pickups.AssertExpectations(t)
}

func TestPickupService_List_CountError(t *testing.T) {
	t.Parallel()

	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	pickups := new(repomock.PickupRepository)
	payments := new(repomock.PaymentRepository)
	svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: time.Now()}, servicetest.Pricing, servicetest.OrganicTTL)

	pickups.On("List", mock.Anything, (*domain.PickupStatus)(nil), (*uuid.UUID)(nil), 20, 0).Return([]domain.Pickup{}, nil).Once()
	pickups.On("Count", mock.Anything, (*domain.PickupStatus)(nil), (*uuid.UUID)(nil)).Return(0, infraErr).Once()

	_, _, err := svc.List(context.Background(), in.PickupFilter{}, pagination.Params{Page: 1, PerPage: 20})

	require.ErrorIs(t, err, infraErr)
	pickups.AssertExpectations(t)
}
