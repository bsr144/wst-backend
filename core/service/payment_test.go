package service_test

import (
	"context"
	"errors"
	"strings"
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
						p.Amount.Equal(decimal.NewFromInt(10000)) &&
						p.PaymentDate == nil &&
						p.ProofFileURL == nil &&
						p.ID != uuid.Nil &&
						p.CreatedAt.Equal(now) &&
						p.UpdatedAt.Equal(now)
				})).Return(nil).Once()
			},
			wantAmount: decimal.NewFromInt(10000),
		},
		{
			name: "electronic pickup creates electronic-priced payment",
			setup: func(payments *repomock.PaymentRepository, pickups *repomock.PickupRepository) {
				pickups.On("FindByID", mock.Anything, wasteID).Return(domain.Pickup{ID: wasteID, HouseholdID: householdID, Type: domain.PickupElectronic, Status: domain.PickupCompleted}, nil).Once()
				payments.On("Insert", mock.Anything, mock.MatchedBy(func(p domain.Payment) bool {
					return p.Amount.Equal(decimal.NewFromInt(50000)) && p.Status == domain.PaymentPending
				})).Return(nil).Once()
			},
			wantAmount: decimal.NewFromInt(50000),
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
			svc := service.NewPaymentService(payments, pickups, storage, fixedClock{now: now}, testPricing)
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
	svc := service.NewPaymentService(payments, pickups, new(repomock.FileStorage), fixedClock{now: now}, testPricing)

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
	svc := service.NewPaymentService(payments, pickups, new(repomock.FileStorage), fixedClock{now: time.Now()}, testPricing)

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
	svc := service.NewPaymentService(payments, pickups, new(repomock.FileStorage), fixedClock{now: time.Now()}, testPricing)

	payments.On("List", mock.Anything, (*domain.PaymentStatus)(nil), (*uuid.UUID)(nil), (*time.Time)(nil), (*time.Time)(nil), 20, 0).Return([]domain.Payment{}, nil).Once()
	payments.On("Count", mock.Anything, (*domain.PaymentStatus)(nil), (*uuid.UUID)(nil), (*time.Time)(nil), (*time.Time)(nil)).Return(0, wantErr).Once()

	_, _, err := svc.List(context.Background(), in.PaymentFilter{}, pagination.Params{Page: 1, PerPage: 20})

	require.ErrorIs(t, err, wantErr)
	payments.AssertExpectations(t)
}

func TestPaymentService_Confirm(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	const url = "http://minio:9000/proofs/payments/x/y.png"
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	storageErr := errors.New("minio down")
	pending := domain.Payment{ID: id, Status: domain.PaymentPending, Amount: decimal.NewFromInt(10000)}
	paid := domain.Payment{ID: id, Status: domain.PaymentPaid, Amount: decimal.NewFromInt(10000), ProofFileURL: ptr(url)}

	keyMatches := mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "payments/"+id.String()+"/") && strings.HasSuffix(key, ".png")
	})

	tests := []struct {
		name      string
		setup     func(*repomock.PaymentRepository, *repomock.FileStorage)
		wantErr   error
		wantCode  string
		noPut     bool
		noConfirm bool
		assertOK  func(*testing.T, domain.Payment)
	}{
		{
			name: "confirms pending payment and stores proof",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(pending, nil).Once()
				storage.On("Put", mock.Anything, keyMatches, mock.Anything, int64(1024), "image/png").Return(url, nil).Once()
				payments.On("Confirm", mock.Anything, id, url, now).Return(paid, true, nil).Once()
			},
			assertOK: func(t *testing.T, got domain.Payment) {
				assert.Equal(t, domain.PaymentPaid, got.Status)
				require.NotNil(t, got.ProofFileURL)
				assert.Equal(t, url, *got.ProofFileURL)
			},
		},
		{
			name: "payment not found",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(domain.Payment{}, domain.ErrPaymentNotFound).Once()
			},
			wantErr:   domain.ErrPaymentNotFound,
			noPut:     true,
			noConfirm: true,
		},
		{
			name: "find infra error propagates",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(domain.Payment{}, infraErr).Once()
			},
			wantErr:   infraErr,
			noPut:     true,
			noConfirm: true,
		},
		{
			name: "already paid is not pending",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(domain.Payment{ID: id, Status: domain.PaymentPaid}, nil).Once()
			},
			wantErr:   domain.ErrPaymentNotPending,
			noPut:     true,
			noConfirm: true,
		},
		{
			name: "storage failure maps to storage unavailable",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(pending, nil).Once()
				storage.On("Put", mock.Anything, keyMatches, mock.Anything, int64(1024), "image/png").Return("", storageErr).Once()
			},
			wantCode:  "STORAGE_UNAVAILABLE",
			noConfirm: true,
		},
		{
			name: "lost race after upload is not pending",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(pending, nil).Once()
				storage.On("Put", mock.Anything, keyMatches, mock.Anything, int64(1024), "image/png").Return(url, nil).Once()
				payments.On("Confirm", mock.Anything, id, url, now).Return(domain.Payment{}, false, nil).Once()
			},
			wantErr: domain.ErrPaymentNotPending,
		},
		{
			name: "confirm infra error propagates",
			setup: func(payments *repomock.PaymentRepository, storage *repomock.FileStorage) {
				payments.On("FindByID", mock.Anything, id).Return(pending, nil).Once()
				storage.On("Put", mock.Anything, keyMatches, mock.Anything, int64(1024), "image/png").Return(url, nil).Once()
				payments.On("Confirm", mock.Anything, id, url, now).Return(domain.Payment{}, false, infraErr).Once()
			},
			wantErr: infraErr,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payments := new(repomock.PaymentRepository)
			pickups := new(repomock.PickupRepository)
			storage := new(repomock.FileStorage)
			svc := service.NewPaymentService(payments, pickups, storage, fixedClock{now: now}, testPricing)
			tc.setup(payments, storage)

			got, err := svc.Confirm(context.Background(), id, in.ConfirmPaymentInput{
				Reader:      strings.NewReader("file-bytes"),
				Size:        1024,
				ContentType: "image/png",
			})

			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			case tc.wantCode != "":
				ae, ok := apperr.From(err)
				require.True(t, ok)
				assert.Equal(t, tc.wantCode, ae.Code)
				require.ErrorIs(t, err, storageErr)
			default:
				require.NoError(t, err)
				tc.assertOK(t, got)
			}
			if tc.noPut {
				storage.AssertNotCalled(t, "Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			if tc.noConfirm {
				payments.AssertNotCalled(t, "Confirm", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			payments.AssertExpectations(t)
			storage.AssertExpectations(t)
		})
	}
}

func ptr[T any](v T) *T { return &v }
