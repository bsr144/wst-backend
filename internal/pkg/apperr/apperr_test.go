package apperr_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"wst-backend/internal/pkg/apperr"
)

func TestKindStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind   apperr.Kind
		status int
		server bool
	}{
		{apperr.KindValidation, 400, false},
		{apperr.KindNotFound, 404, false},
		{apperr.KindConflict, 409, false},
		{apperr.KindUnprocessable, 422, false},
		{apperr.KindRateLimited, 429, false},
		{apperr.KindUnavailable, 503, true},
		{apperr.KindInternal, 500, true},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.status, tc.kind.Status())
		assert.Equal(t, tc.server, tc.kind.IsServerError())
	}
}

func TestClientCodeAndMessage_HidesInternalDetail(t *testing.T) {
	t.Parallel()

	internal := apperr.Internal("DB_OOPS", "duplicate key value violates unique constraint")

	assert.Equal(t, apperr.CodeInternal, internal.ClientCode())
	assert.Equal(t, apperr.MessageFor(apperr.CodeInternal), internal.ClientMessage())
	assert.NotContains(t, internal.ClientMessage(), "constraint")
}

func TestClientCodeAndMessage_PreservesClientFacing(t *testing.T) {
	t.Parallel()

	conflict := apperr.Conflict("PICKUP_NOT_PENDING", "pickup must be pending for this action")

	assert.Equal(t, "PICKUP_NOT_PENDING", conflict.ClientCode())
	assert.Equal(t, "pickup must be pending for this action", conflict.ClientMessage())
}

func TestCause_RoundTrips(t *testing.T) {
	t.Parallel()

	root := errors.New("boom")
	wrapped := apperr.Internal(apperr.CodeInternal, "database error").WithCause(root)

	assert.Equal(t, root, wrapped.Cause())
	assert.ErrorIs(t, wrapped, root)
}

func TestMessageFor_FallsBackToInternal(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "validation failed", apperr.MessageFor(apperr.CodeValidation))
	assert.Equal(t, apperr.MessageFor(apperr.CodeInternal), apperr.MessageFor("UNREGISTERED_CODE"))
}
