package pickup_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/pickup"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
)

func TestPickupService_Complete(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	householdID := uuid.New()

	t.Run("electronic completes and creates electronic-priced payment", func(t *testing.T) {
		t.Parallel()
		completed := domain.Pickup{ID: id, HouseholdID: householdID, Type: domain.PickupElectronic, Status: domain.PickupCompleted, CreatedAt: now, UpdatedAt: now}
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
		pickups.On("Complete", mock.Anything, id, now).Return(completed, true, nil).Once()
		payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
			return p.HouseholdID == householdID &&
				p.WasteID == id &&
				p.Status == domain.PaymentPending &&
				p.Amount.Equal(decimal.NewFromInt(100000)) &&
				p.PaymentDate == nil &&
				p.ProofFileURL == nil &&
				p.ID != uuid.Nil &&
				p.CreatedAt.Equal(now)
		})).Return(nil).Once()

		got, err := svc.Complete(context.Background(), id)

		require.NoError(t, err)
		assert.Equal(t, domain.PickupCompleted, got.Status)
		pickups.AssertExpectations(t)
		payments.AssertExpectations(t)
	})

	t.Run("standard type uses standard price", func(t *testing.T) {
		t.Parallel()
		completed := domain.Pickup{ID: id, HouseholdID: householdID, Type: domain.PickupPlastic, Status: domain.PickupCompleted, CreatedAt: now, UpdatedAt: now}
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
		pickups.On("Complete", mock.Anything, id, now).Return(completed, true, nil).Once()
		payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
			return p.Amount.Equal(decimal.NewFromInt(50000)) && p.Status == domain.PaymentPending
		})).Return(nil).Once()

		_, err := svc.Complete(context.Background(), id)

		require.NoError(t, err)
		payments.AssertExpectations(t)
	})

	t.Run("not scheduled does not create payment", func(t *testing.T) {
		t.Parallel()
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
		pickups.On("Complete", mock.Anything, id, now).Return(domain.Pickup{}, false, nil).Once()
		pickups.On("FindByID", mock.Anything, id).Return(domain.Pickup{ID: id, Status: domain.PickupPending}, nil).Once()

		_, err := svc.Complete(context.Background(), id)

		require.ErrorIs(t, err, domain.ErrPickupNotScheduled)
		payments.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		pickups.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
		pickups.On("Complete", mock.Anything, id, now).Return(domain.Pickup{}, false, nil).Once()
		pickups.On("FindByID", mock.Anything, id).Return(domain.Pickup{}, domain.ErrPickupNotFound).Once()

		_, err := svc.Complete(context.Background(), id)

		require.ErrorIs(t, err, domain.ErrPickupNotFound)
		payments.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		pickups.AssertExpectations(t)
	})
}

func TestPickupService_Complete_Errors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	householdID := uuid.New()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("complete repo error propagates and skips payment", func(t *testing.T) {
		t.Parallel()
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
		pickups.On("Complete", mock.Anything, id, now).Return(domain.Pickup{}, false, infraErr).Once()

		_, err := svc.Complete(context.Background(), id)

		require.ErrorIs(t, err, infraErr)
		payments.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		pickups.AssertExpectations(t)
	})

	t.Run("payment insert error fails the transaction", func(t *testing.T) {
		t.Parallel()
		completed := domain.Pickup{ID: id, HouseholdID: householdID, Type: domain.PickupOrganic, Status: domain.PickupCompleted, CreatedAt: now, UpdatedAt: now}
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
		pickups.On("Complete", mock.Anything, id, now).Return(completed, true, nil).Once()
		payments.On("Insert", mock.Anything, mock.Anything).Return(infraErr).Once()

		_, err := svc.Complete(context.Background(), id)

		require.ErrorIs(t, err, infraErr)
		pickups.AssertExpectations(t)
		payments.AssertExpectations(t)
	})
}
