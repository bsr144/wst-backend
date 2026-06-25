package postgres

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"wst-backend/internal/pkg/apperr"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.From(err); ok {
		return err
	}
	if isUnavailable(err) {
		return apperr.Unavailable(apperr.CodeServiceUnavailable, apperr.MessageFor(apperr.CodeServiceUnavailable)).WithCause(err)
	}
	return apperr.Internal(apperr.CodeInternal, "database error").WithCause(err)
}

func isUnavailable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "08") || strings.HasPrefix(pgErr.Code, "57")
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}
