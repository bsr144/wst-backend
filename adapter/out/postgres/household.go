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

type householdRow struct {
	ID        uuid.UUID `db:"id"`
	OwnerName string    `db:"owner_name"`
	Address   string    `db:"address"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r householdRow) toDomain() domain.Household {
	return domain.Household{
		ID:        r.ID,
		OwnerName: r.OwnerName,
		Address:   r.Address,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

type HouseholdRepository struct {
	pool *pgxpool.Pool
}

func NewHouseholdRepository(pool *pgxpool.Pool) *HouseholdRepository {
	return &HouseholdRepository{pool: pool}
}

func (r *HouseholdRepository) Insert(ctx context.Context, h domain.Household) error {
	const query = `INSERT INTO households (id, owner_name, address, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := Executor(ctx, r.pool).Exec(ctx, query, h.ID, h.OwnerName, h.Address, h.CreatedAt, h.UpdatedAt)
	return err
}

func (r *HouseholdRepository) List(ctx context.Context, limit, offset int) ([]domain.Household, error) {
	const query = `SELECT id, owner_name, address, created_at, updated_at FROM households ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[householdRow])
	if err != nil {
		return nil, err
	}
	households := make([]domain.Household, 0, len(collected))
	for _, row := range collected {
		households = append(households, row.toDomain())
	}
	return households, nil
}

func (r *HouseholdRepository) Count(ctx context.Context) (int, error) {
	const query = `SELECT count(*) FROM households`
	var total int
	if err := Executor(ctx, r.pool).QueryRow(ctx, query).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *HouseholdRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Household, error) {
	const query = `SELECT id, owner_name, address, created_at, updated_at FROM households WHERE id = $1`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, id)
	if err != nil {
		return domain.Household{}, err
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[householdRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Household{}, domain.ErrHouseholdNotFound
		}
		return domain.Household{}, err
	}
	return row.toDomain(), nil
}

func (r *HouseholdRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM households WHERE id = $1`
	tag, err := Executor(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.ErrHouseholdHasDependents
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrHouseholdNotFound
	}
	return nil
}

var _ out.HouseholdRepository = (*HouseholdRepository)(nil)
