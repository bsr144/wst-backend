package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"wst-backend/core/domain"
	"wst-backend/core/port/out"
)

type PaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) HasPendingByHousehold(ctx context.Context, householdID uuid.UUID) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM payments WHERE household_id = $1 AND status = 'pending')`
	var exists bool
	if err := Executor(ctx, r.pool).QueryRow(ctx, query, householdID).Scan(&exists); err != nil {
		return false, mapError(err)
	}
	return exists, nil
}

func (r *PaymentRepository) Insert(ctx context.Context, p domain.Payment) error {
	const query = `INSERT INTO payments (id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := Executor(ctx, r.pool).Exec(ctx, query, p.ID, p.HouseholdID, p.WasteID, p.Amount, p.PaymentDate, string(p.Status), p.ProofFileURL, p.CreatedAt, p.UpdatedAt)
	return mapError(err)
}

var _ out.PaymentRepository = (*PaymentRepository)(nil)
