package report_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/report"
	"wst-backend/internal/pkg/apperr"
)

func TestReportService_WasteSummary(t *testing.T) {
	t.Parallel()
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("aggregates and sums total", func(t *testing.T) {
		t.Parallel()
		reports := new(repomock.ReportRepository)
		households := new(repomock.HouseholdRepository)
		svc := report.NewService(reports, households)
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
		svc := report.NewService(reports, households)
		reports.On("WasteSummary", mock.Anything).Return(nil, infraErr).Once()

		_, err := svc.WasteSummary(context.Background())

		require.ErrorIs(t, err, infraErr)
		reports.AssertExpectations(t)
	})
}
