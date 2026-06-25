package report_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/report"
	"wst-backend/internal/pkg/apperr"
)

func TestReportService_PaymentSummary(t *testing.T) {
	t.Parallel()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("revenue counts only paid amounts", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := report.NewService(reports, households)
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
		svc := report.NewService(reports, households)
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
		svc := report.NewService(reports, households)
		reports.On("PaymentSummary", mock.Anything).Return(nil, infraErr).Once()

		_, err := svc.PaymentSummary(context.Background())

		require.ErrorIs(t, err, infraErr)
		reports.AssertExpectations(t)
	})
}
