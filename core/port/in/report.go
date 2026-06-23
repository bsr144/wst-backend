package in

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/core/domain"
)

type ReportService interface {
	WasteSummary(ctx context.Context) (domain.WasteSummary, error)
	PaymentSummary(ctx context.Context) (domain.PaymentSummary, error)
	HouseholdHistory(ctx context.Context, householdID uuid.UUID) (domain.HouseholdHistory, error)
}
