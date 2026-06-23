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

	httpx "wst-backend/adapter/in/http"
	"wst-backend/adapter/in/http/handler"
	"wst-backend/adapter/in/http/presenter"
	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	"wst-backend/pkg/apperr"
	"wst-backend/pkg/pagination"
)

type paymentServiceMock struct {
	mock.Mock
}

func (m *paymentServiceMock) Create(ctx context.Context, cmd in.CreatePaymentCommand) (domain.Payment, error) {
	args := m.Called(ctx, cmd)
	var p domain.Payment
	if v := args.Get(0); v != nil {
		p = v.(domain.Payment)
	}
	return p, args.Error(1)
}

func (m *paymentServiceMock) List(ctx context.Context, filter in.PaymentFilter, params pagination.Params) ([]domain.Payment, int, error) {
	args := m.Called(ctx, filter, params)
	var items []domain.Payment
	if v := args.Get(0); v != nil {
		items = v.([]domain.Payment)
	}
	return items, args.Int(1), args.Error(2)
}

var _ in.PaymentService = (*paymentServiceMock)(nil)

func newPaymentTestApp(svc in.PaymentService) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return presenter.Error(c, err)
		},
	})
	httpx.RegisterPaymentRoutes(app.Group("/api"), handler.NewPaymentHandler(svc))
	return app
}

func TestPaymentHandler_Create_MalformedJSON(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", "{not json")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "VALIDATION_ERROR", body["error"].(map[string]any)["code"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentHandler_Create_MissingWasteID(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	assert.Equal(t, "waste_id", details[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentHandler_Create_BadHouseholdID(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"not-a-uuid","waste_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	assert.Equal(t, "household_id", errBody["details"].([]any)[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentHandler_Create_BadWasteID(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+uuid.New().String()+`","waste_id":"nope"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	assert.Equal(t, "waste_id", errBody["details"].([]any)[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentHandler_Create_Success(t *testing.T) {
	t.Parallel()
	now := mustTime()
	id := uuid.New()
	householdID := uuid.New()
	wasteID := uuid.New()
	created := domain.Payment{ID: id, HouseholdID: householdID, WasteID: wasteID, Amount: decimal.NewFromInt(10000), Status: domain.PaymentPending, CreatedAt: now, UpdatedAt: now}

	svc := new(paymentServiceMock)
	svc.On("Create", mock.Anything, mock.MatchedBy(func(cmd in.CreatePaymentCommand) bool {
		return cmd.HouseholdID == householdID && cmd.WasteID == wasteID
	})).Return(created, nil).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+householdID.String()+`","waste_id":"`+wasteID.String()+`"}`)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, id.String(), data["id"])
	assert.Equal(t, householdID.String(), data["household_id"])
	assert.Equal(t, wasteID.String(), data["waste_id"])
	assert.Equal(t, "10000.00", data["amount"])
	assert.Equal(t, "pending", data["status"])
	assert.Nil(t, data["payment_date"])
	assert.Nil(t, data["proof_file_url"])
	svc.AssertExpectations(t)
}

func TestPaymentHandler_Create_HouseholdMismatch(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	svc.On("Create", mock.Anything, mock.Anything).Return(domain.Payment{}, domain.ErrPaymentHouseholdMismatch).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+uuid.New().String()+`","waste_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "PAYMENT_HOUSEHOLD_MISMATCH", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPaymentHandler_Create_AlreadyExists(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	svc.On("Create", mock.Anything, mock.Anything).Return(domain.Payment{}, domain.ErrPaymentAlreadyExists).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+uuid.New().String()+`","waste_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "PAYMENT_ALREADY_EXISTS", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPaymentHandler_Create_PickupNotFound(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	svc.On("Create", mock.Anything, mock.Anything).Return(domain.Payment{}, domain.ErrPickupNotFound).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+uuid.New().String()+`","waste_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "PICKUP_NOT_FOUND", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPaymentHandler_Create_Unavailable(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	svc.On("Create", mock.Anything, mock.Anything).Return(domain.Payment{}, apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/payments", `{"household_id":"`+uuid.New().String()+`","waste_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "SERVICE_UNAVAILABLE", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPaymentHandler_List_Success(t *testing.T) {
	t.Parallel()
	now := mustTime()
	householdID := uuid.New()
	items := []domain.Payment{
		{ID: uuid.New(), HouseholdID: householdID, WasteID: uuid.New(), Amount: decimal.NewFromInt(50000), Status: domain.PaymentPaid, CreatedAt: now, UpdatedAt: now},
	}
	svc := new(paymentServiceMock)
	svc.On("List", mock.Anything, mock.MatchedBy(func(f in.PaymentFilter) bool {
		return f.Status != nil && *f.Status == domain.PaymentPaid &&
			f.HouseholdID != nil && *f.HouseholdID == householdID &&
			f.DateFrom != nil && f.DateTo != nil
	}), pagination.Params{Page: 1, PerPage: 20}).Return(items, 1, nil).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/payments?status=paid&household_id="+householdID.String()+"&date_from=2026-06-01T00:00:00Z&date_to=2026-06-30T00:00:00Z", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].([]any)
	assert.Len(t, data, 1)
	assert.Equal(t, "50000.00", data[0].(map[string]any)["amount"])
	assert.Equal(t, float64(1), body["meta"].(map[string]any)["total"])
	svc.AssertExpectations(t)
}

func TestPaymentHandler_List_InvalidStatusFilter(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/payments?status=bogus", "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	assert.Equal(t, "status", errBody["details"].([]any)[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
}

func TestPaymentHandler_List_BadDate(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/payments?date_from=2026-13-01", "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	assert.Equal(t, "date_from", errBody["details"].([]any)[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
}

func TestPaymentHandler_List_DateFromAfterDateTo(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/payments?date_from=2026-06-30T00:00:00Z&date_to=2026-06-01T00:00:00Z", "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	assert.Equal(t, "date_from", errBody["details"].([]any)[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
}

func TestPaymentHandler_List_Defaults(t *testing.T) {
	t.Parallel()
	svc := new(paymentServiceMock)
	svc.On("List", mock.Anything, in.PaymentFilter{}, pagination.Params{Page: 1, PerPage: 20}).
		Return([]domain.Payment{}, 0, nil).Once()
	app := newPaymentTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/payments", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, float64(0), body["meta"].(map[string]any)["total"])
	svc.AssertExpectations(t)
}
