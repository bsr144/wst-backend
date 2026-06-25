package middleware_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"wst-backend/internal/adapter/in/http/middleware"
	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/pkg/apperr"
)

func newObservedApp() (*fiber.App, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return presenter.Error(c, err)
		},
	})
	app.Use(middleware.RequestID(logger))
	app.Use(middleware.Recover())
	app.Use(middleware.AccessLog())

	app.Get("/boom", func(c *fiber.Ctx) error {
		cause := errors.New("pq: duplicate key value violates unique constraint \"payments_waste_id_key\"")
		return presenter.Error(c, apperr.Internal(apperr.CodeInternal, "database error").WithCause(cause))
	})
	app.Get("/bad", func(c *fiber.Ctx) error {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "validation failed").
			WithDetails(apperr.FieldError{Field: "type", Reason: "is required"}))
	})
	app.Get("/panic", func(c *fiber.Ctx) error {
		panic("unexpected nil dereference")
	})

	return app, logs
}

func fire(t *testing.T, app *fiber.App, target string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", target, nil), -1)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return resp.StatusCode, body
}

func only(t *testing.T, logs *observer.ObservedLogs, msg string) observer.LoggedEntry {
	t.Helper()
	entries := logs.FilterMessage(msg).All()
	require.Len(t, entries, 1, "expected exactly one %q log line", msg)
	return entries[0]
}

func TestServerError_GenericToClient_CauseLogged(t *testing.T) {
	t.Parallel()
	app, logs := newObservedApp()

	status, body := fire(t, app, "/boom")

	assert.Equal(t, fiber.StatusInternalServerError, status)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, apperr.CodeInternal, errBody["code"])
	assert.Equal(t, apperr.MessageFor(apperr.CodeInternal), errBody["message"])
	assert.NotContains(t, errBody["message"], "duplicate key")
	assert.NotContains(t, errBody["message"], "payments_waste_id_key")

	failed := only(t, logs, "request_failed")
	assert.Equal(t, zapcore.ErrorLevel, failed.Level)
	fields := failed.ContextMap()
	assert.NotEmpty(t, fields["request_id"])
	assert.Contains(t, fields["error"], "duplicate key value violates unique constraint")
	assert.EqualValues(t, 500, fields["status"])

	access := only(t, logs, "http_request")
	assert.Equal(t, fields["request_id"], access.ContextMap()["request_id"])
	assert.EqualValues(t, 500, access.ContextMap()["status"])
}

func TestClientError_ClearMessage_CauseLogged(t *testing.T) {
	t.Parallel()
	app, logs := newObservedApp()

	status, body := fire(t, app, "/bad")

	assert.Equal(t, fiber.StatusBadRequest, status)
	errBody := body["error"].(map[string]any)
	assert.Equal(t, apperr.CodeValidation, errBody["code"])
	assert.Equal(t, "validation failed", errBody["message"])
	require.Len(t, errBody["details"], 1)

	rejected := only(t, logs, "request_rejected")
	assert.Equal(t, zapcore.WarnLevel, rejected.Level)
	fields := rejected.ContextMap()
	assert.NotEmpty(t, fields["request_id"])
	assert.Equal(t, apperr.CodeValidation, fields["code"])
	assert.NotNil(t, fields["details"])
}

func TestPanic_Recovered_LoggedWithRequestID(t *testing.T) {
	t.Parallel()
	app, logs := newObservedApp()

	status, body := fire(t, app, "/panic")

	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Equal(t, apperr.CodeInternal, body["error"].(map[string]any)["code"])

	panicEntry := only(t, logs, "panic_recovered")
	assert.Equal(t, zapcore.ErrorLevel, panicEntry.Level)
	assert.NotEmpty(t, panicEntry.ContextMap()["request_id"])
	assert.Contains(t, panicEntry.ContextMap()["panic"], "nil dereference")
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	t.Parallel()
	app, logs := newObservedApp()

	fire(t, app, "/bad")
	fire(t, app, "/bad")

	accessLines := logs.FilterMessage("http_request").All()
	require.Len(t, accessLines, 2)
	first := accessLines[0].ContextMap()["request_id"]
	second := accessLines[1].ContextMap()["request_id"]
	assert.NotEmpty(t, first)
	assert.NotEmpty(t, second)
	assert.NotEqual(t, first, second)
}

func TestRequestID_HonorsInboundHeader(t *testing.T) {
	t.Parallel()
	app, logs := newObservedApp()

	req := httptest.NewRequest("GET", "/bad", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, "trace-abc-123", resp.Header.Get("X-Request-ID"))

	assert.Equal(t, "trace-abc-123", only(t, logs, "http_request").ContextMap()["request_id"])
	assert.Equal(t, "trace-abc-123", only(t, logs, "request_rejected").ContextMap()["request_id"])
}
