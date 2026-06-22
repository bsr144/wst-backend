package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"wst-backend/core/domain"
	"wst-backend/core/port/out"
)

type pickupRow struct {
	ID          uuid.UUID  `db:"id"`
	HouseholdID uuid.UUID  `db:"household_id"`
	Type        string     `db:"type"`
	Status      string     `db:"status"`
	PickupDate  *time.Time `db:"pickup_date"`
	SafetyCheck bool       `db:"safety_check"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (r pickupRow) toDomain() domain.Pickup {
	return domain.Pickup{
		ID:          r.ID,
		HouseholdID: r.HouseholdID,
		Type:        domain.PickupType(r.Type),
		Status:      domain.PickupStatus(r.Status),
		PickupDate:  r.PickupDate,
		SafetyCheck: r.SafetyCheck,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type PickupRepository struct {
	pool *pgxpool.Pool
}

func NewPickupRepository(pool *pgxpool.Pool) *PickupRepository {
	return &PickupRepository{pool: pool}
}

func (r *PickupRepository) Insert(ctx context.Context, p domain.Pickup) error {
	const query = `INSERT INTO waste_pickups (id, household_id, type, status, pickup_date, safety_check, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := Executor(ctx, r.pool).Exec(ctx, query, p.ID, p.HouseholdID, string(p.Type), string(p.Status), p.PickupDate, p.SafetyCheck, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.ErrHouseholdNotFound
		}
		return mapError(err)
	}
	return nil
}

func (r *PickupRepository) List(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID, limit, offset int) ([]domain.Pickup, error) {
	const query = `SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at FROM waste_pickups WHERE ($1::text IS NULL OR status = $1) AND ($2::uuid IS NULL OR household_id = $2) ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, statusArg(status), householdID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[pickupRow])
	if err != nil {
		return nil, mapError(err)
	}
	pickups := make([]domain.Pickup, 0, len(collected))
	for _, row := range collected {
		pickups = append(pickups, row.toDomain())
	}
	return pickups, nil
}

func (r *PickupRepository) Count(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID) (int, error) {
	const query = `SELECT count(*) FROM waste_pickups WHERE ($1::text IS NULL OR status = $1) AND ($2::uuid IS NULL OR household_id = $2)`
	var total int
	if err := Executor(ctx, r.pool).QueryRow(ctx, query, statusArg(status), householdID).Scan(&total); err != nil {
		return 0, mapError(err)
	}
	return total, nil
}

func (r *PickupRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
	const query = `SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at FROM waste_pickups WHERE id = $1`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, id)
	if err != nil {
		return domain.Pickup{}, mapError(err)
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[pickupRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pickup{}, domain.ErrPickupNotFound
		}
		return domain.Pickup{}, mapError(err)
	}
	return row.toDomain(), nil
}

func (r *PickupRepository) Schedule(ctx context.Context, id uuid.UUID, pickupDate, now time.Time) (domain.Pickup, bool, error) {
	const query = `UPDATE waste_pickups SET status = 'scheduled', pickup_date = $2, updated_at = $3 WHERE id = $1 AND status = 'pending' AND (type <> 'electronic' OR safety_check = true) RETURNING id, household_id, type, status, pickup_date, safety_check, created_at, updated_at`
	return r.returningOne(ctx, query, id, pickupDate, now)
}

func (r *PickupRepository) Complete(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error) {
	const query = `UPDATE waste_pickups SET status = 'completed', updated_at = $2 WHERE id = $1 AND status = 'scheduled' RETURNING id, household_id, type, status, pickup_date, safety_check, created_at, updated_at`
	return r.returningOne(ctx, query, id, now)
}

func (r *PickupRepository) Cancel(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error) {
	const query = `UPDATE waste_pickups SET status = 'canceled', updated_at = $2 WHERE id = $1 AND status IN ('pending', 'scheduled') RETURNING id, household_id, type, status, pickup_date, safety_check, created_at, updated_at`
	return r.returningOne(ctx, query, id, now)
}

var _ out.PickupRepository = (*PickupRepository)(nil)
