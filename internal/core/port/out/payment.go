package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

type PaymentReader interface {
	HasPendingByHousehold(ctx context.Context, householdID uuid.UUID) (bool, error)
	List(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time, limit, offset int) ([]domain.Payment, error)
	Count(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time) (int, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Payment, error)
}

type PaymentWriter interface {
	Insert(ctx context.Context, p domain.Payment) error
	Confirm(ctx context.Context, id uuid.UUID, proofURL string, now time.Time) (domain.Payment, bool, error)
}

type PaymentRepository interface {
	PaymentReader
	PaymentWriter
}
