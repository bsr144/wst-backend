package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	httpx "wst-backend/internal/adapter/in/http"
	"wst-backend/internal/adapter/in/http/handler"
	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/pkg/pagination"
)

type pickupServiceMock struct {
	mock.Mock
}

func (m *pickupServiceMock) Create(ctx context.Context, cmd in.CreatePickupCommand) (domain.Pickup, error) {
	args := m.Called(ctx, cmd)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Error(1)
}

func (m *pickupServiceMock) List(ctx context.Context, filter in.PickupFilter, params pagination.Params) ([]domain.Pickup, int, error) {
	args := m.Called(ctx, filter, params)
	var items []domain.Pickup
	if v := args.Get(0); v != nil {
		items = v.([]domain.Pickup)
	}
	return items, args.Int(1), args.Error(2)
}

func (m *pickupServiceMock) Schedule(ctx context.Context, id uuid.UUID, cmd in.SchedulePickupCommand) (domain.Pickup, error) {
	args := m.Called(ctx, id, cmd)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Error(1)
}

func (m *pickupServiceMock) Complete(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
	args := m.Called(ctx, id)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Error(1)
}

func (m *pickupServiceMock) Cancel(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
	args := m.Called(ctx, id)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Error(1)
}

func (m *pickupServiceMock) CancelStaleOrganic(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

var _ in.PickupService = (*pickupServiceMock)(nil)

func newPickupTestApp(svc in.PickupService) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return presenter.Error(c, err)
		},
	})
	passthrough := func(c *fiber.Ctx) error { return c.Next() }
	httpx.RegisterPickupRoutes(app.Group("/api"), handler.NewPickupHandler(svc), passthrough)
	return app
}

func TestPickupHandler_Create_MalformedJSON(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", "{not json")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPickupHandler_Create_MissingType(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"`+uuid.New().String()+`"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	first := details[0].(map[string]any)
	assert.Equal(t, "type", first["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPickupHandler_Create_ElectronicMissingSafetyCheck(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"`+uuid.New().String()+`","type":"electronic"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	first := details[0].(map[string]any)
	assert.Equal(t, "safety_check", first["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPickupHandler_Create_BadHouseholdID(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"not-a-uuid","type":"plastic"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	first := details[0].(map[string]any)
	assert.Equal(t, "household_id", first["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPickupHandler_Create_PlasticNoSafetyCheck(t *testing.T) {
	t.Parallel()
	now := mustTime()
	id := uuid.New()
	householdID := uuid.New()
	created := domain.Pickup{ID: id, HouseholdID: householdID, Type: domain.PickupPlastic, Status: domain.PickupPending, CreatedAt: now, UpdatedAt: now}

	svc := new(pickupServiceMock)
	svc.On("Create", mock.Anything, mock.MatchedBy(func(cmd in.CreatePickupCommand) bool {
		return cmd.HouseholdID == householdID && cmd.Type == domain.PickupPlastic && !cmd.SafetyCheck
	})).Return(created, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"`+householdID.String()+`","type":"plastic"}`)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, id.String(), data["id"])
	assert.Equal(t, householdID.String(), data["household_id"])
	assert.Equal(t, "plastic", data["type"])
	assert.Equal(t, "pending", data["status"])
	assert.Equal(t, false, data["safety_check"])
	assert.Nil(t, data["pickup_date"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Create_ElectronicSuccess(t *testing.T) {
	t.Parallel()
	now := mustTime()
	id := uuid.New()
	householdID := uuid.New()
	created := domain.Pickup{ID: id, HouseholdID: householdID, Type: domain.PickupElectronic, Status: domain.PickupPending, SafetyCheck: true, CreatedAt: now, UpdatedAt: now}

	svc := new(pickupServiceMock)
	svc.On("Create", mock.Anything, mock.MatchedBy(func(cmd in.CreatePickupCommand) bool {
		return cmd.HouseholdID == householdID && cmd.Type == domain.PickupElectronic && cmd.SafetyCheck
	})).Return(created, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"`+householdID.String()+`","type":"electronic","safety_check":true}`)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, "electronic", data["type"])
	assert.Equal(t, true, data["safety_check"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Create_PendingPayment(t *testing.T) {
	t.Parallel()
	householdID := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Create", mock.Anything, mock.Anything).Return(domain.Pickup{}, domain.ErrHouseholdHasPendingPayment).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"`+householdID.String()+`","type":"organic"}`)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "HOUSEHOLD_HAS_PENDING_PAYMENT", errBody["code"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Create_HouseholdNotFound(t *testing.T) {
	t.Parallel()
	householdID := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Create", mock.Anything, mock.Anything).Return(domain.Pickup{}, domain.ErrHouseholdNotFound).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/pickups", `{"household_id":"`+householdID.String()+`","type":"paper"}`)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "HOUSEHOLD_NOT_FOUND", errBody["code"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_List_Success(t *testing.T) {
	t.Parallel()
	now := mustTime()
	householdID := uuid.New()
	items := []domain.Pickup{
		{ID: uuid.New(), HouseholdID: householdID, Type: domain.PickupOrganic, Status: domain.PickupPending, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), HouseholdID: householdID, Type: domain.PickupPaper, Status: domain.PickupPending, CreatedAt: now, UpdatedAt: now},
	}
	svc := new(pickupServiceMock)
	svc.On("List", mock.Anything, mock.MatchedBy(func(f in.PickupFilter) bool {
		return f.Status != nil && *f.Status == domain.PickupPending &&
			f.HouseholdID != nil && *f.HouseholdID == householdID
	}), pagination.Params{Page: 1, PerPage: 10}).Return(items, 2, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/pickups?status=pending&household_id="+householdID.String(), "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].([]any)
	assert.Len(t, data, 2)
	meta := body["meta"].(map[string]any)
	assert.Equal(t, float64(2), meta["total"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_List_InvalidStatusFilter(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/pickups?status=bogus", "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	first := details[0].(map[string]any)
	assert.Equal(t, "status", first["field"])
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
}

func TestPickupHandler_List_Defaults(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	svc.On("List", mock.Anything, in.PickupFilter{}, pagination.Params{Page: 1, PerPage: 10}).
		Return([]domain.Pickup{}, 0, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/pickups", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	meta := body["meta"].(map[string]any)
	assert.Equal(t, float64(0), meta["total"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Schedule_Success(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := mustTime()
	pd := mustTime()
	scheduled := domain.Pickup{ID: id, Type: domain.PickupOrganic, Status: domain.PickupScheduled, PickupDate: &pd, CreatedAt: now, UpdatedAt: now}

	svc := new(pickupServiceMock)
	svc.On("Schedule", mock.Anything, id, mock.MatchedBy(func(cmd in.SchedulePickupCommand) bool {
		return cmd.PickupDate.Equal(pd)
	})).Return(scheduled, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/schedule", `{"pickup_date":"2026-06-21T10:00:00Z"}`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, "scheduled", data["status"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Schedule_MissingDate(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/schedule", `{}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	assert.Equal(t, "pickup_date", details[0].(map[string]any)["field"])
	svc.AssertNotCalled(t, "Schedule", mock.Anything, mock.Anything, mock.Anything)
}

func TestPickupHandler_Schedule_BadID(t *testing.T) {
	t.Parallel()
	svc := new(pickupServiceMock)
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/not-a-uuid/schedule", `{"pickup_date":"2026-06-21T10:00:00Z"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "VALIDATION_ERROR", body["error"].(map[string]any)["code"])
	svc.AssertNotCalled(t, "Schedule", mock.Anything, mock.Anything, mock.Anything)
}

func TestPickupHandler_Schedule_SafetyCheckRequired(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Schedule", mock.Anything, id, mock.Anything).Return(domain.Pickup{}, domain.ErrSafetyCheckRequired).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/schedule", `{"pickup_date":"2026-06-21T10:00:00Z"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "SAFETY_CHECK_REQUIRED", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Complete_Success(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := mustTime()
	completed := domain.Pickup{ID: id, Type: domain.PickupPlastic, Status: domain.PickupCompleted, CreatedAt: now, UpdatedAt: now}

	svc := new(pickupServiceMock)
	svc.On("Complete", mock.Anything, id).Return(completed, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/complete", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "completed", body["data"].(map[string]any)["status"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Complete_NotScheduled(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Complete", mock.Anything, id).Return(domain.Pickup{}, domain.ErrPickupNotScheduled).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/complete", "")

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "PICKUP_NOT_SCHEDULED", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Cancel_Success(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := mustTime()
	canceled := domain.Pickup{ID: id, Type: domain.PickupOrganic, Status: domain.PickupCanceled, CreatedAt: now, UpdatedAt: now}

	svc := new(pickupServiceMock)
	svc.On("Cancel", mock.Anything, id).Return(canceled, nil).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/cancel", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "canceled", body["data"].(map[string]any)["status"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Cancel_NotCancelable(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Cancel", mock.Anything, id).Return(domain.Pickup{}, domain.ErrPickupNotCancelable).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/cancel", "")

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "PICKUP_NOT_CANCELABLE", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Schedule_NotPending(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Schedule", mock.Anything, id, mock.Anything).Return(domain.Pickup{}, domain.ErrPickupNotPending).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/schedule", `{"pickup_date":"2026-06-21T10:00:00Z"}`)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "PICKUP_NOT_PENDING", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}

func TestPickupHandler_Complete_NotFound(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(pickupServiceMock)
	svc.On("Complete", mock.Anything, id).Return(domain.Pickup{}, domain.ErrPickupNotFound).Once()
	app := newPickupTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPut, "/api/pickups/"+id.String()+"/complete", "")

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "PICKUP_NOT_FOUND", body["error"].(map[string]any)["code"])
	svc.AssertExpectations(t)
}
