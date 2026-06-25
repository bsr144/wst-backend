package out

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

type ReportReader interface {
	WasteSummary(ctx context.Context) ([]domain.WasteCount, error)
	PaymentSummary(ctx context.Context) ([]domain.PaymentStatusTotal, error)
	HouseholdPickups(ctx context.Context, householdID uuid.UUID) ([]domain.Pickup, error)
	HouseholdPayments(ctx context.Context, householdID uuid.UUID) ([]domain.Payment, error)
}

type ReportRepository interface {
	ReportReader
}
