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

func TestPickupService_Create(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	householdID := uuid.New()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	tests := []struct {
		name     string
		cmd      in.CreatePickupCommand
		setup    func(*repomock.PickupRepository, *repomock.PaymentRepository)
		wantErr  error
		noInsert bool
		assertOK func(*testing.T, domain.Pickup)
	}{
		{
			name: "electronic with safety check",
			cmd:  in.CreatePickupCommand{HouseholdID: householdID, Type: domain.PickupElectronic, SafetyCheck: true},
			setup: func(pickups *repomock.PickupRepository, payments *repomock.PaymentRepository) {
				payments.On("HasPendingByHousehold", mock.Anything, householdID).Return(false, nil).Once()
				pickups.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Pickup) bool {
					return p.HouseholdID == householdID &&
						p.Type == domain.PickupElectronic &&
						p.Status == domain.PickupPending &&
						p.SafetyCheck &&
						p.PickupDate == nil &&
						p.CreatedAt.Equal(now) &&
						p.UpdatedAt.Equal(now) &&
						p.ID != uuid.Nil
				})).Return(nil).Once()
			},
			assertOK: func(t *testing.T, got domain.Pickup) {
				assert.Equal(t, domain.PickupPending, got.Status)
				assert.Equal(t, domain.PickupElectronic, got.Type)
				assert.Equal(t, householdID, got.HouseholdID)
				assert.True(t, got.SafetyCheck)
				assert.Nil(t, got.PickupDate)
				assert.NotEqual(t, uuid.Nil, got.ID)
			},
		},
		{
			name: "pending payment blocks creation",
			cmd:  in.CreatePickupCommand{HouseholdID: householdID, Type: domain.PickupOrganic},
			setup: func(pickups *repomock.PickupRepository, payments *repomock.PaymentRepository) {
				payments.On("HasPendingByHousehold", mock.Anything, householdID).Return(true, nil).Once()
			},
			wantErr:  domain.ErrHouseholdHasPendingPayment,
			noInsert: true,
		},
		{
			name: "insert fk maps to household not found",
			cmd:  in.CreatePickupCommand{HouseholdID: householdID, Type: domain.PickupPlastic},
			setup: func(pickups *repomock.PickupRepository, payments *repomock.PaymentRepository) {
				payments.On("HasPendingByHousehold", mock.Anything, householdID).Return(false, nil).Once()
				pickups.On("Insert", mock.Anything, mock.Anything).Return(domain.ErrHouseholdNotFound).Once()
			},
			wantErr: domain.ErrHouseholdNotFound,
		},
		{
			name: "pending check infra error propagates",
			cmd:  in.CreatePickupCommand{HouseholdID: householdID, Type: domain.PickupPaper},
			setup: func(pickups *repomock.PickupRepository, payments *repomock.PaymentRepository) {
				payments.On("HasPendingByHousehold", mock.Anything, householdID).Return(false, infraErr).Once()
			},
			wantErr:  infraErr,
			noInsert: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pickups := new(repomock.PickupRepository)
			payments := new(repomock.PaymentRepository)
			svc := pickup.NewService(pickups, payments, servicetest.PassthroughTx{}, servicetest.FixedClock{At: now}, servicetest.Pricing, servicetest.OrganicTTL)
			tc.setup(pickups, payments)

			got, err := svc.Create(context.Background(), tc.cmd)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				tc.assertOK(t, got)
			}
			if tc.noInsert {
				pickups.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
			}
			pickups.AssertExpectations(t)
			payments.AssertExpectations(t)
		})
	}
}
