package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

type serviceMock struct {
	mock.Mock
}

func (m *serviceMock) Create(ctx context.Context, cmd in.CreateHouseholdCommand) (domain.Household, error) {
	args := m.Called(ctx, cmd)
	var h domain.Household
	if v := args.Get(0); v != nil {
		h = v.(domain.Household)
	}
	return h, args.Error(1)
}

func (m *serviceMock) List(ctx context.Context, params pagination.Params) ([]domain.Household, int, error) {
	args := m.Called(ctx, params)
	var items []domain.Household
	if v := args.Get(0); v != nil {
		items = v.([]domain.Household)
	}
	return items, args.Int(1), args.Error(2)
}

func (m *serviceMock) Get(ctx context.Context, id uuid.UUID) (domain.Household, error) {
	args := m.Called(ctx, id)
	var h domain.Household
	if v := args.Get(0); v != nil {
		h = v.(domain.Household)
	}
	return h, args.Error(1)
}

func (m *serviceMock) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

var _ in.HouseholdService = (*serviceMock)(nil)

func newTestApp(svc in.HouseholdService) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return presenter.Error(c, err)
		},
	})
	httpx.RegisterRoutes(app.Group("/api"), handler.NewHouseholdHandler(svc))
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, target, body string) (*http.Response, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var decoded map[string]any
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &decoded))
	}
	return resp, decoded
}

func TestHouseholdHandler_Create_MalformedJSON(t *testing.T) {
	t.Parallel()
	svc := new(serviceMock)
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/households", "{not json")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	assert.Equal(t, "invalid request body", errBody["message"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHouseholdHandler_Create_MissingOwnerName(t *testing.T) {
	t.Parallel()
	svc := new(serviceMock)
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/households", `{"address":"Jl. Mawar 1"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	first := details[0].(map[string]any)
	assert.Equal(t, "owner_name", first["field"])
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHouseholdHandler_Create_Success(t *testing.T) {
	t.Parallel()
	now := mustTime()
	id := uuid.New()
	created := domain.Household{ID: id, OwnerName: "Budi", Address: "Jl. Mawar 1", CreatedAt: now, UpdatedAt: now}

	svc := new(serviceMock)
	svc.On("Create", mock.Anything, in.CreateHouseholdCommand{OwnerName: "Budi", Address: "Jl. Mawar 1"}).
		Return(created, nil).Once()
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodPost, "/api/households", `{"owner_name":"Budi","address":"Jl. Mawar 1"}`)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	data := body["data"].(map[string]any)
	assert.Equal(t, id.String(), data["id"])
	assert.Equal(t, "Budi", data["owner_name"])
	assert.Equal(t, "Jl. Mawar 1", data["address"])
	assert.NotEmpty(t, data["created_at"])
	assert.NotEmpty(t, data["updated_at"])
	_, hasMeta := body["meta"]
	assert.False(t, hasMeta)
	svc.AssertExpectations(t)
}

func TestHouseholdHandler_List_Success(t *testing.T) {
	t.Parallel()
	now := mustTime()
	items := []domain.Household{
		{ID: uuid.New(), OwnerName: "Budi", Address: "Jl. Mawar 1", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), OwnerName: "Siti", Address: "Jl. Melati 2", CreatedAt: now, UpdatedAt: now},
	}
	svc := new(serviceMock)
	svc.On("List", mock.Anything, pagination.Params{Page: 1, PerPage: 10}).Return(items, 2, nil).Once()
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/households", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].([]any)
	assert.Len(t, data, 2)
	meta := body["meta"].(map[string]any)
	assert.Equal(t, float64(1), meta["page"])
	assert.Equal(t, float64(10), meta["per_page"])
	assert.Equal(t, float64(2), meta["total"])
	svc.AssertExpectations(t)
}

func TestHouseholdHandler_Get_BadUUID(t *testing.T) {
	t.Parallel()
	svc := new(serviceMock)
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/households/not-a-uuid", "")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
	details := errBody["details"].([]any)
	require.NotEmpty(t, details)
	first := details[0].(map[string]any)
	assert.Equal(t, "id", first["field"])
	svc.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

func TestHouseholdHandler_Get_NotFound(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(serviceMock)
	svc.On("Get", mock.Anything, id).Return(domain.Household{}, domain.ErrHouseholdNotFound).Once()
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodGet, "/api/households/"+id.String(), "")

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "HOUSEHOLD_NOT_FOUND", errBody["code"])
	svc.AssertExpectations(t)
}

func TestHouseholdHandler_Delete_Dependents(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(serviceMock)
	svc.On("Delete", mock.Anything, id).Return(domain.ErrHouseholdHasDependents).Once()
	app := newTestApp(svc)

	resp, body := doRequest(t, app, http.MethodDelete, "/api/households/"+id.String(), "")

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "HOUSEHOLD_HAS_DEPENDENTS", errBody["code"])
	svc.AssertExpectations(t)
}

func TestHouseholdHandler_Delete_Success(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := new(serviceMock)
	svc.On("Delete", mock.Anything, id).Return(nil).Once()
	app := newTestApp(svc)

	resp, _ := doRequest(t, app, http.MethodDelete, "/api/households/"+id.String(), "")

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	svc.AssertExpectations(t)
}

func mustTime() time.Time {
	return time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
}
