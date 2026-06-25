package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	httpx "wst-backend/internal/adapter/in/http"
	"wst-backend/internal/adapter/in/http/handler"
	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/pkg/apperr"
)

type reportServiceMock struct {
	mock.Mock
}

func (m *reportServiceMock) WasteSummary(ctx context.Context) (domain.WasteSummary, error) {
	args := m.Called(ctx)
	var v domain.WasteSummary
	if r := args.Get(0); r != nil {
		v = r.(domain.WasteSummary)
	}
	return v, args.Error(1)
}

func (m *reportServiceMock) PaymentSummary(ctx context.Context) (domain.PaymentSummary, error) {
	args := m.Called(ctx)
	var v domain.PaymentSummary
	if r := args.Get(0); r != nil {
		v = r.(domain.PaymentSummary)
	}
	return v, args.Error(1)
}

func (m *reportServiceMock) HouseholdHistory(ctx context.Context, householdID uuid.UUID) (domain.HouseholdHistory, error) {
	args := m.Called(ctx, householdID)
	var v domain.HouseholdHistory
	if r := args.Get(0); r != nil {
		v = r.(domain.HouseholdHistory)
	}
	return v, args.Error(1)
}

var _ in.ReportService = (*reportServiceMock)(nil)

func newReportTestApp(svc in.ReportService) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return presenter.Error(c, err)
		},
	})
	httpx.RegisterReportRoutes(app.Group("/api"), handler.NewReportHandler(svc))
	return app
}

func TestReportHandler_WasteSummary(t *testing.T) {
	t.Parallel()
	svc := new(reportServiceMock)
	app := newReportTestApp(svc)
	svc.On("WasteSummary", mock.Anything).Return(domain.WasteSummary{
		Counts: []domain.WasteCount{{Type: domain.PickupOrganic, Status: domain.PickupPending, Count: 3}},
		Total:  3,
	}, nil).Once()

	resp, body := doRequest(t, app, http.MethodGet, "/api/reports/waste-summary", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(3), data["total"])
	counts := data["counts"].([]any)
	require.Len(t, counts, 1)
	first := counts[0].(map[string]any)
	assert.Equal(t, "organic", first["type"])
	assert.Equal(t, "pending", first["status"])
	assert.Equal(t, float64(3), first["count"])
	svc.AssertExpectations(t)
}

func TestReportHandler_PaymentSummary(t *testing.T) {
	t.Parallel()
	svc := new(reportServiceMock)
	app := newReportTestApp(svc)
	svc.On("PaymentSummary", mock.Anything).Return(domain.PaymentSummary{
		Totals:       []domain.PaymentStatusTotal{{Status: domain.PaymentPaid, Count: 4, Amount: decimal.NewFromInt(70000)}},
		TotalRevenue: decimal.NewFromInt(70000),
	}, nil).Once()

	resp, body := doRequest(t, app, http.MethodGet, "/api/reports/payment-summary", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, "70000.00", data["total_revenue"])
	totals := data["totals"].([]any)
	require.Len(t, totals, 1)
	first := totals[0].(map[string]any)
	assert.Equal(t, "paid", first["status"])
	assert.Equal(t, "70000.00", first["amount"])
	svc.AssertExpectations(t)
}

func TestReportHandler_HouseholdHistory(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(reportServiceMock)
	app := newReportTestApp(svc)
	svc.On("HouseholdHistory", mock.Anything, id).Return(domain.HouseholdHistory{
		Household: domain.Household{ID: id, OwnerName: "x", Address: "y"},
		Pickups:   []domain.Pickup{{ID: uuid.New(), HouseholdID: id, Type: domain.PickupPaper, Status: domain.PickupPending}},
		Payments:  []domain.Payment{{ID: uuid.New(), HouseholdID: id, Amount: decimal.NewFromInt(10000), Status: domain.PaymentPending}},
	}, nil).Once()

	resp, body := doRequest(t, app, http.MethodGet, "/api/reports/households/"+id.String()+"/history", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]any)
	household := data["household"].(map[string]any)
	assert.Equal(t, id.String(), household["id"])
	assert.Len(t, data["pickups"].([]any), 1)
	assert.Len(t, data["payments"].([]any), 1)
	svc.AssertExpectations(t)
}

func TestReportHandler_HouseholdHistory_InvalidID(t *testing.T) {
	t.Parallel()
	svc := new(reportServiceMock)
	app := newReportTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/reports/households/not-a-uuid/history", "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	svc.AssertNotCalled(t, "HouseholdHistory", mock.Anything, mock.Anything)
}

func TestReportHandler_HouseholdHistory_NotFound(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(reportServiceMock)
	app := newReportTestApp(svc)
	svc.On("HouseholdHistory", mock.Anything, id).Return(domain.HouseholdHistory{}, domain.ErrHouseholdNotFound).Once()

	resp, body := doRequest(t, app, http.MethodGet, "/api/reports/households/"+id.String()+"/history", "")

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "HOUSEHOLD_NOT_FOUND", errBody["code"])
	svc.AssertExpectations(t)
}

func TestReportHandler_WasteSummary_ServiceUnavailable(t *testing.T) {
	t.Parallel()
	svc := new(reportServiceMock)
	app := newReportTestApp(svc)
	svc.On("WasteSummary", mock.Anything).Return(domain.WasteSummary{}, apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")).Once()

	resp, body := doRequest(t, app, http.MethodGet, "/api/reports/waste-summary", "")

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "SERVICE_UNAVAILABLE", errBody["code"])
	svc.AssertExpectations(t)
}
