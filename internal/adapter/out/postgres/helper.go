package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"wst-backend/internal/core/domain"
)

func statusArg(status *domain.PickupStatus) *string {
	if status == nil {
		return nil
	}
	s := string(*status)
	return &s
}

func paymentStatusArg(status *domain.PaymentStatus) *string {
	if status == nil {
		return nil
	}
	s := string(*status)
	return &s
}

func (r *PickupRepository) returningOne(ctx context.Context, query string, args ...any) (domain.Pickup, bool, error) {
	rows, err := Executor(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return domain.Pickup{}, false, mapError(err)
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[pickupRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pickup{}, false, nil
		}
		return domain.Pickup{}, false, mapError(err)
	}
	return row.toDomain(), true, nil
}
