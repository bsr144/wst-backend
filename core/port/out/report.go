package out

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/core/domain"
)

type ReportRepository interface {
	WasteSummary(ctx context.Context) ([]domain.WasteCount, error)
	PaymentSummary(ctx context.Context) ([]domain.PaymentStatusTotal, error)
	HouseholdPickups(ctx context.Context, householdID uuid.UUID) ([]domain.Pickup, error)
	HouseholdPayments(ctx context.Context, householdID uuid.UUID) ([]domain.Payment, error)
}
