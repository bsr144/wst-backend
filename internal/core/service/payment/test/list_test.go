package payment_test

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
	"wst-backend/internal/core/port/in"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/payment"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
	"wst-backend/internal/pkg/pagination"
)

func TestPaymentService_List(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	householdID := uuid.New()
	status := domain.PaymentPaid
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	items := []domain.Payment{
		{ID: uuid.New(), HouseholdID: householdID, WasteID: uuid.New(), Amount: decimal.NewFromInt(10000), Status: domain.PaymentPaid, CreatedAt: now, UpdatedAt: now},
	}
	payments := new(repomock.PaymentRepository)
	pickups := new(repomock.PickupRepository)
	svc := payment.NewService(payments, pickups, new(repomock.FileStorage), servicetest.FixedClock{At: now}, servicetest.Pricing)

	params := pagination.Params{Page: 2, PerPage: 10}
	payments.On("List", mock.Anything, &status, &householdID, &from, &to, 10, 10).Return(items, nil).Once()
	payments.On("Count", mock.Anything, &status, &householdID, &from, &to).Return(3, nil).Once()

	got, total, err := svc.List(context.Background(), in.PaymentFilter{Status: &status, HouseholdID: &householdID, DateFrom: &from, DateTo: &to}, params)

	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, 3, total)
	payments.AssertExpectations(t)
}

func TestPaymentService_List_Error(t *testing.T) {
	t.Parallel()

	wantErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	payments := new(repomock.PaymentRepository)
	pickups := new(repomock.PickupRepository)
	svc := payment.NewService(payments, pickups, new(repomock.FileStorage), servicetest.FixedClock{At: time.Now()}, servicetest.Pricing)

	payments.On("List", mock.Anything, (*domain.PaymentStatus)(nil), (*uuid.UUID)(nil), (*time.Time)(nil), (*time.Time)(nil), 20, 0).Return(nil, wantErr).Once()

	_, _, err := svc.List(context.Background(), in.PaymentFilter{}, pagination.Params{Page: 1, PerPage: 20})

	require.ErrorIs(t, err, wantErr)
	payments.AssertExpectations(t)
}

func TestPaymentService_List_CountError(t *testing.T) {
	t.Parallel()

	wantErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	payments := new(repomock.PaymentRepository)
	pickups := new(repomock.PickupRepository)
	svc := payment.NewService(payments, pickups, new(repomock.FileStorage), servicetest.FixedClock{At: time.Now()}, servicetest.Pricing)

	payments.On("List", mock.Anything, (*domain.PaymentStatus)(nil), (*uuid.UUID)(nil), (*time.Time)(nil), (*time.Time)(nil), 20, 0).Return([]domain.Payment{}, nil).Once()
	payments.On("Count", mock.Anything, (*domain.PaymentStatus)(nil), (*uuid.UUID)(nil), (*time.Time)(nil), (*time.Time)(nil)).Return(0, wantErr).Once()

	_, _, err := svc.List(context.Background(), in.PaymentFilter{}, pagination.Params{Page: 1, PerPage: 20})

	require.ErrorIs(t, err, wantErr)
	payments.AssertExpectations(t)
}
