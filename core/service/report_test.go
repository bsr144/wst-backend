package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/core/domain"
	repomock "wst-backend/core/port/out/mock"
	"wst-backend/core/service"
	"wst-backend/pkg/apperr"
)

func TestReportService_WasteSummary(t *testing.T) {
	t.Parallel()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("aggregates and sums total", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := service.NewReportService(reports, households)
		counts := []domain.WasteCount{
			{Type: domain.PickupOrganic, Status: domain.PickupPending, Count: 3},
			{Type: domain.PickupOrganic, Status: domain.PickupCompleted, Count: 2},
			{Type: domain.PickupElectronic, Status: domain.PickupScheduled, Count: 5},
		}
		reports.On("WasteSummary", mock.Anything).Return(counts, nil).Once()

		got, err := svc.WasteSummary(context.Background())

		require.NoError(t, err)
		assert.Equal(t, counts, got.Counts)
		assert.Equal(t, 10, got.Total)
		reports.AssertExpectations(t)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := service.NewReportService(reports, households)
		reports.On("WasteSummary", mock.Anything).Return(nil, infraErr).Once()

		_, err := svc.WasteSummary(context.Background())

		require.ErrorIs(t, err, infraErr)
		reports.AssertExpectations(t)
	})
}

func TestReportService_PaymentSummary(t *testing.T) {
	t.Parallel()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("revenue counts only paid amounts", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := service.NewReportService(reports, households)
		totals := []domain.PaymentStatusTotal{
			{Status: domain.PaymentPending, Count: 2, Amount: decimal.NewFromInt(20000)},
			{Status: domain.PaymentPaid, Count: 4, Amount: decimal.NewFromInt(70000)},
			{Status: domain.PaymentFailed, Count: 1, Amount: decimal.NewFromInt(10000)},
		}
		reports.On("PaymentSummary", mock.Anything).Return(totals, nil).Once()

		got, err := svc.PaymentSummary(context.Background())

		require.NoError(t, err)
		assert.Equal(t, totals, got.Totals)
		assert.True(t, got.TotalRevenue.Equal(decimal.NewFromInt(70000)), "revenue was %s", got.TotalRevenue)
		reports.AssertExpectations(t)
	})

	t.Run("no paid rows yields zero revenue", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := service.NewReportService(reports, households)
		totals := []domain.PaymentStatusTotal{
			{Status: domain.PaymentPending, Count: 2, Amount: decimal.NewFromInt(20000)},
		}
		reports.On("PaymentSummary", mock.Anything).Return(totals, nil).Once()

		got, err := svc.PaymentSummary(context.Background())

		require.NoError(t, err)
		assert.True(t, got.TotalRevenue.Equal(decimal.Zero))
		reports.AssertExpectations(t)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := service.NewReportService(reports, households)
		reports.On("PaymentSummary", mock.Anything).Return(nil, infraErr).Once()

		_, err := svc.PaymentSummary(context.Background())

		require.ErrorIs(t, err, infraErr)
		reports.AssertExpectations(t)
	})
}

func TestReportService_HouseholdHistory(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("composes household, pickups, and payments", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := service.NewReportService(reports, households)
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
		svc := service.NewReportService(reports, households)
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
		svc := service.NewReportService(reports, households)
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
		svc := service.NewReportService(reports, households)
		households.On("FindByID", mock.Anything, id).Return(domain.Household{ID: id}, nil).Once()
		reports.On("HouseholdPickups", mock.Anything, id).Return([]domain.Pickup{}, nil).Once()
		reports.On("HouseholdPayments", mock.Anything, id).Return(nil, infraErr).Once()

		_, err := svc.HouseholdHistory(context.Background(), id)

		require.ErrorIs(t, err, infraErr)
		households.AssertExpectations(t)
		reports.AssertExpectations(t)
	})
}
