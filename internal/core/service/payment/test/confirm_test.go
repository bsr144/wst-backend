package payment_test

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

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/payment"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
)

func ptr[T any](v T) *T { return &v }

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
			svc := payment.NewService(payments, pickups, storage, servicetest.FixedClock{At: now}, servicetest.Pricing)
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

func TestPaymentService_Confirm_ContentTypeExtensions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		contentType string
		wantSuffix  string
	}{
		{name: "jpeg maps to .jpg", contentType: "image/jpeg", wantSuffix: ".jpg"},
		{name: "pdf maps to .pdf", contentType: "application/pdf", wantSuffix: ".pdf"},
		{name: "unknown type yields no extension", contentType: "image/webp", wantSuffix: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id := uuid.New()
			pending := domain.Payment{ID: id, Status: domain.PaymentPending, Amount: decimal.NewFromInt(10000)}
			payments := new(repomock.PaymentRepository)
			pickups := new(repomock.PickupRepository)
			storage := new(repomock.FileStorage)
			svc := payment.NewService(payments, pickups, storage, servicetest.FixedClock{At: now}, servicetest.Pricing)

			keyMatches := mock.MatchedBy(func(key string) bool {
				if !strings.HasPrefix(key, "payments/"+id.String()+"/") {
					return false
				}
				if tc.wantSuffix == "" {
					return !strings.Contains(key, ".")
				}
				return strings.HasSuffix(key, tc.wantSuffix)
			})
			url := "http://minio:9000/proofs/" + id.String()
			payments.On("FindByID", mock.Anything, id).Return(pending, nil).Once()
			storage.On("Put", mock.Anything, keyMatches, mock.Anything, int64(1024), tc.contentType).Return(url, nil).Once()
			payments.On("Confirm", mock.Anything, id, url, now).Return(domain.Payment{ID: id, Status: domain.PaymentPaid}, true, nil).Once()

			got, err := svc.Confirm(context.Background(), id, in.ConfirmPaymentInput{
				Reader:      strings.NewReader("file-bytes"),
				Size:        1024,
				ContentType: tc.contentType,
			})

			require.NoError(t, err)
			assert.Equal(t, domain.PaymentPaid, got.Status)
			payments.AssertExpectations(t)
			storage.AssertExpectations(t)
		})
	}
}
