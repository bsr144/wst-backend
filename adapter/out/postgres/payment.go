package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"wst-backend/core/domain"
	"wst-backend/core/port/out"
)

type paymentRow struct {
	ID           uuid.UUID      `db:"id"`
	HouseholdID  uuid.UUID      `db:"household_id"`
	WasteID      uuid.UUID      `db:"waste_id"`
	Amount       pgtype.Numeric `db:"amount"`
	PaymentDate  *time.Time     `db:"payment_date"`
	Status       string         `db:"status"`
	ProofFileURL *string        `db:"proof_file_url"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

func (r paymentRow) toDomain() domain.Payment {
	return domain.Payment{
		ID:           r.ID,
		HouseholdID:  r.HouseholdID,
		WasteID:      r.WasteID,
		Amount:       numericToDecimal(r.Amount),
		PaymentDate:  r.PaymentDate,
		Status:       domain.PaymentStatus(r.Status),
		ProofFileURL: r.ProofFileURL,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func numericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(n.Int, n.Exp)
}

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
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrPaymentAlreadyExists
		}
		return mapError(err)
	}
	return nil
}

func (r *PaymentRepository) List(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time, limit, offset int) ([]domain.Payment, error) {
	const query = `SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at FROM payments WHERE ($1::text IS NULL OR status = $1) AND ($2::uuid IS NULL OR household_id = $2) AND ($3::timestamptz IS NULL OR payment_date >= $3) AND ($4::timestamptz IS NULL OR payment_date <= $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, paymentStatusArg(status), householdID, dateFrom, dateTo, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		return nil, mapError(err)
	}
	payments := make([]domain.Payment, 0, len(collected))
	for _, row := range collected {
		payments = append(payments, row.toDomain())
	}
	return payments, nil
}

func (r *PaymentRepository) Count(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time) (int, error) {
	const query = `SELECT count(*) FROM payments WHERE ($1::text IS NULL OR status = $1) AND ($2::uuid IS NULL OR household_id = $2) AND ($3::timestamptz IS NULL OR payment_date >= $3) AND ($4::timestamptz IS NULL OR payment_date <= $4)`
	var total int
	if err := Executor(ctx, r.pool).QueryRow(ctx, query, paymentStatusArg(status), householdID, dateFrom, dateTo).Scan(&total); err != nil {
		return 0, mapError(err)
	}
	return total, nil
}

func (r *PaymentRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Payment, error) {
	const query = `SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at FROM payments WHERE id = $1`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, id)
	if err != nil {
		return domain.Payment{}, mapError(err)
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Payment{}, domain.ErrPaymentNotFound
		}
		return domain.Payment{}, mapError(err)
	}
	return row.toDomain(), nil
}

func (r *PaymentRepository) Confirm(ctx context.Context, id uuid.UUID, proofURL string, now time.Time) (domain.Payment, bool, error) {
	const query = `UPDATE payments SET status = 'paid', proof_file_url = $2, payment_date = $3, updated_at = $3 WHERE id = $1 AND status = 'pending' RETURNING id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, id, proofURL, now)
	if err != nil {
		return domain.Payment{}, false, mapError(err)
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Payment{}, false, nil
		}
		return domain.Payment{}, false, mapError(err)
	}
	return row.toDomain(), true, nil
}

var _ out.PaymentRepository = (*PaymentRepository)(nil)
