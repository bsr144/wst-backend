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
)

func TestPaymentService_Create(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	householdID := uuid.New()
	wasteID := uuid.New()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	tests := []struct {
		name       string
		setup      func(*repomock.PaymentRepository, *repomock.PickupRepository)
		wantErr    error
		noInsert   bool
		wantAmount decimal.Decimal
	}{
		{
			name: "standard pickup creates standard-priced pending payment",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{ID: wasteID, HouseholdID: householdID, Type: domain.PickupPlastic, Status: domain.PickupCompleted}, nil).Once()
				payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
					return p.HouseholdID == householdID &&
						p.WasteID == wasteID &&
						p.Status == domain.PaymentPending &&
						p.Amount.Equal(decimal.NewFromInt(50000)) &&
						p.PaymentDate == nil &&
						p.ProofFileURL == nil &&
						p.ID != uuid.Nil &&
						p.CreatedAt.Equal(now) &&
						p.UpdatedAt.Equal(now)
				})).Return(nil).Once()
			},
			wantAmount: decimal.NewFromInt(50000),
		},
		{
			name: "electronic pickup creates electronic-priced payment",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{ID: wasteID, HouseholdID: householdID, Type: domain.PickupElectronic, Status: domain.PickupCompleted}, nil).Once()
				payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
					return p.Amount.Equal(decimal.NewFromInt(100000)) && p.Status == domain.PaymentPending
				})).Return(nil).Once()
			},
			wantAmount: decimal.NewFromInt(100000),
		},
		{
			name: "pickup not found",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{}, domain.ErrPickupNotFound).Once()
			},
			wantErr:  domain.ErrPickupNotFound,
			noInsert: true,
		},
		{
			name: "household mismatch",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{ID: wasteID, HouseholdID: uuid.New(), Type: domain.PickupOrganic}, nil).Once()
			},
			wantErr:  domain.ErrPaymentHouseholdMismatch,
			noInsert: true,
		},
		{
			name: "already exists propagates from repo",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{ID: wasteID, HouseholdID: householdID, Type: domain.PickupPaper}, nil).Once()
				payments.On("Insert", mock.Anything, mock.Anything).Return(domain.ErrPaymentAlreadyExists).Once()
			},
			wantErr: domain.ErrPaymentAlreadyExists,
		},
		{
			name: "insert infra error propagates",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{ID: wasteID, HouseholdID: householdID, Type: domain.PickupOrganic}, nil).Once()
				payments.On("Insert", mock.Anything, mock.Anything).Return(infraErr).Once()
			},
			wantErr: infraErr,
		},
		{
			name: "find infra error propagates",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{}, infraErr).Once()
			},
			wantErr:  infraErr,
			noInsert: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payments := new(repomock.PaymentRepository)
			pickups := new(repomock.PickupRepository)
			storage := new(repomock.FileStorage)
			svc := payment.NewService(payments, pickups, storage, servicetest.FixedClock{At: now}, servicetest.Pricing)
			tc.setup(payments, pickups)

			got, err := svc.Create(context.Background(), in.CreatePaymentCommand{HouseholdID: householdID, WasteID: wasteID})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, domain.PaymentPending, got.Status)
				assert.Equal(t, householdID, got.HouseholdID)
				assert.Equal(t, wasteID, got.WasteID)
				assert.True(t, got.Amount.Equal(tc.wantAmount))
				assert.NotEqual(t, uuid.Nil, got.ID)
			}
			if tc.noInsert {
				payments.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
			}
			payments.AssertExpectations(t)
			pickups.AssertExpectations(t)
		})
	}
}
