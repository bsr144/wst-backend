package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/out"
)

type wasteCountRow struct {
	Type   string `db:"type"`
	Status string `db:"status"`
	Count  int    `db:"count"`
}

type paymentTotalRow struct {
	Status string         `db:"status"`
	Count  int            `db:"count"`
	Amount pgtype.Numeric `db:"amount"`
}

type ReportRepository struct {
	pool *pgxpool.Pool
}

func NewReportRepository(pool *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{pool: pool}
}

func (r *ReportRepository) WasteSummary(ctx context.Context) ([]domain.WasteCount, error) {
	const query = `SELECT type, status, count(*)::int AS count FROM waste_pickups GROUP BY type, status ORDER BY type, status`
	rows, err := Executor(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, mapError(err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[wasteCountRow])
	if err != nil {
		return nil, mapError(err)
	}
	counts := make([]domain.WasteCount, 0, len(collected))
	for _, row := range collected {
		counts = append(counts, domain.WasteCount{
			Type:   domain.PickupType(row.Type),
			Status: domain.PickupStatus(row.Status),
			Count:  row.Count,
		})
	}
	return counts, nil
}

func (r *ReportRepository) PaymentSummary(ctx context.Context) ([]domain.PaymentStatusTotal, error) {
	const query = `SELECT status, count(*)::int AS count, coalesce(sum(amount), 0) AS amount FROM payments GROUP BY status ORDER BY status`
	rows, err := Executor(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, mapError(err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[paymentTotalRow])
	if err != nil {
		return nil, mapError(err)
	}
	totals := make([]domain.PaymentStatusTotal, 0, len(collected))
	for _, row := range collected {
		totals = append(totals, domain.PaymentStatusTotal{
			Status: domain.PaymentStatus(row.Status),
			Count:  row.Count,
			Amount: numericToDecimal(row.Amount),
		})
	}
	return totals, nil
}

func (r *ReportRepository) HouseholdPickups(ctx context.Context, householdID uuid.UUID) ([]domain.Pickup, error) {
	const query = `SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at FROM waste_pickups WHERE household_id = $1 ORDER BY created_at DESC`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, householdID)
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

func (r *ReportRepository) HouseholdPayments(ctx context.Context, householdID uuid.UUID) ([]domain.Payment, error) {
	const query = `SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at FROM payments WHERE household_id = $1 ORDER BY created_at DESC`
	rows, err := Executor(ctx, r.pool).Query(ctx, query, householdID)
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

var (
	_ out.ReportRepository = (*ReportRepository)(nil)
	_ out.ReportReader     = (*ReportRepository)(nil)
)
