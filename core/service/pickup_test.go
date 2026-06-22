package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	repomock "wst-backend/core/port/out/mock"
	"wst-backend/core/service"
	"wst-backend/pkg/apperr"
	"wst-backend/pkg/pagination"
)

type passthroughTx struct{}

func (passthroughTx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

var testPricing = domain.Pricing{Standard: decimal.NewFromInt(10000), Electronic: decimal.NewFromInt(50000)}

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
			svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
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
	svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)

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
	svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: time.Now()}, testPricing)

	pickups.On("List", mock.Anything, (*domain.PickupStatus)(nil), (*uuid.UUID)(nil), 20, 0).Return(nil, wantErr).Once()

	_, _, err := svc.List(context.Background(), in.PickupFilter{}, pagination.Params{Page: 1, PerPage: 20})

	require.ErrorIs(t, err, wantErr)
	pickups.AssertExpectations(t)
}

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
			svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
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
		svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
		pickups.On("Complete", mock.Anything, id, now).Return(completed, true, nil).Once()
		payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
			return p.HouseholdID == householdID &&
				p.WasteID == id &&
				p.Status == domain.PaymentPending &&
				p.Amount.Equal(decimal.NewFromInt(50000)) &&
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
		svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
		pickups.On("Complete", mock.Anything, id, now).Return(completed, true, nil).Once()
		payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
			return p.Amount.Equal(decimal.NewFromInt(10000)) && p.Status == domain.PaymentPending
		})).Return(nil).Once()

		_, err := svc.Complete(context.Background(), id)

		require.NoError(t, err)
		payments.AssertExpectations(t)
	})

	t.Run("not scheduled does not create payment", func(t *testing.T) {
		t.Parallel()
		pickups := new(repomock.PickupRepository)
		payments := new(repomock.PaymentRepository)
		svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
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
		svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
		pickups.On("Complete", mock.Anything, id, now).Return(domain.Pickup{}, false, nil).Once()
		pickups.On("FindByID", mock.Anything, id).Return(domain.Pickup{}, domain.ErrPickupNotFound).Once()

		_, err := svc.Complete(context.Background(), id)

		require.ErrorIs(t, err, domain.ErrPickupNotFound)
		payments.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		pickups.AssertExpectations(t)
	})
}

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
			svc := service.NewPickupService(pickups, payments, passthroughTx{}, fixedClock{now: now}, testPricing)
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
