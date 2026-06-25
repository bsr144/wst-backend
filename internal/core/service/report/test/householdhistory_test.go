package report_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/report"
	"wst-backend/internal/pkg/apperr"
)

func TestReportService_HouseholdHistory(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("composes household, pickups, and payments", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := report.NewService(reports, households)
		household := domain.Household{ID: id, OwnerName: "x", Address: "y"}
		pickups := []domain.Pickup{{ID: uuid.New(), HouseholdID: id, Type: domain.PickupPaper, Status: domain.PickupPending}}
		payments := []domain.Payment{{ID: uuid.New(), HouseholdID: id, Amount: decimal.NewFromInt(10000), Status: domain.PaymentPending}}
		households.On("FindByID", mock.Anything, id).Return(household, nil).Once()
		reports.On("HouseholdPickups", mock.Anything, id).Return(pickups, nil).Once()
		reports.On("HouseholdPayments", mock.Anything, id).Return(payments, nil).Once()

		got, err := svc.HouseholdHistory(context.Background(), id)

		require.NoError(t, err)
		assert.Equal(t, household, got.Household)
		assert.Equal(t, pickups, got.Pickups)
		assert.Equal(t, payments, got.Payments)
		households.AssertExpectations(t)
		reports.AssertExpectations(t)
	})

	t.Run("not found short-circuits before history queries", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := report.NewService(reports, households)
		households.On("FindByID", mock.Anything, id).Return(domain.Household{}, domain.ErrHouseholdNotFound).Once()

		_, err := svc.HouseholdHistory(context.Background(), id)

		require.ErrorIs(t, err, domain.ErrHouseholdNotFound)
		reports.AssertNotCalled(t, "HouseholdPickups", mock.Anything, mock.Anything)
		reports.AssertNotCalled(t, "HouseholdPayments", mock.Anything, mock.Anything)
		households.AssertExpectations(t)
	})

	t.Run("pickups repo error propagates", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := report.NewService(reports, households)
		households.On("FindByID", mock.Anything, id).Return(domain.Household{ID: id}, nil).Once()
		reports.On("HouseholdPickups", mock.Anything, id).Return(nil, infraErr).Once()

		_, err := svc.HouseholdHistory(context.Background(), id)

		require.ErrorIs(t, err, infraErr)
		reports.AssertNotCalled(t, "HouseholdPayments", mock.Anything, mock.Anything)
		households.AssertExpectations(t)
		reports.AssertExpectations(t)
	})

	t.Run("payments repo error propagates", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := report.NewService(reports, households)
		households.On("FindByID", mock.Anything, id).Return(domain.Household{ID: id}, nil).Once()
		reports.On("HouseholdPickups", mock.Anything, id).Return([]domain.Pickup{}, nil).Once()
		reports.On("HouseholdPayments", mock.Anything, id).Return(nil, infraErr).Once()

		_, err := svc.HouseholdHistory(context.Background(), id)

		require.ErrorIs(t, err, infraErr)
		households.AssertExpectations(t)
		reports.AssertExpectations(t)
	})
}
