package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wst-backend/pkg/apperr"
)

type timeoutNetErr struct{}

func (timeoutNetErr) Error() string   { return "i/o timeout" }
func (timeoutNetErr) Timeout() bool   { return true }
func (timeoutNetErr) Temporary() bool { return true }

func TestMapError_ClassifiesInfraErrors(t *testing.T) {
	t.Parallel()

	fkViolation := &pgconn.PgError{Code: "23503"}
	domainConflict := apperr.Conflict("X_CONFLICT", "x")
	generic := errors.New("boom")

	tests := []struct {
		name        string
		err         error
		wantNil     bool
		unavailable bool
		internal    bool
		same        error
	}{
		{name: "nil stays nil", err: nil, wantNil: true},
		{name: "deadline exceeded", err: context.DeadlineExceeded, unavailable: true},
		{name: "canceled", err: context.Canceled, unavailable: true},
		{name: "wrapped deadline", err: fmt.Errorf("query failed: %w", context.DeadlineExceeded), unavailable: true},
		{name: "pg connection class 08", err: &pgconn.PgError{Code: "08006"}, unavailable: true},
		{name: "pg operator intervention class 57", err: &pgconn.PgError{Code: "57014"}, unavailable: true},
		{name: "pg connect error", err: &pgconn.ConnectError{}, unavailable: true},
		{name: "raw net error", err: timeoutNetErr{}, unavailable: true},
		{name: "unclassified pg error wraps to internal", err: fkViolation, internal: true},
		{name: "domain apperr passes through", err: domainConflict, same: domainConflict},
		{name: "generic error wraps to internal", err: generic, internal: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mapError(tc.err)

			switch {
			case tc.wantNil:
				assert.NoError(t, got)
			case tc.unavailable:
				ae, ok := apperr.From(got)
				require.True(t, ok)
				assert.Equal(t, apperr.KindUnavailable, ae.Kind)
				assert.Equal(t, "SERVICE_UNAVAILABLE", ae.Code)
				assert.ErrorIs(t, got, tc.err)
			case tc.internal:
				ae, ok := apperr.From(got)
				require.True(t, ok)
				assert.Equal(t, apperr.KindInternal, ae.Kind)
				assert.Equal(t, "INTERNAL_ERROR", ae.Code)
				assert.ErrorIs(t, got, tc.err)
			default:
				assert.Equal(t, tc.same, got)
			}
		})
	}
}
