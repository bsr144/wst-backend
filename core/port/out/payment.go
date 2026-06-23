package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/core/domain"
)

type PaymentRepository interface {
	HasPendingByHousehold(ctx context.Context, householdID uuid.UUID) (bool, error)
	Insert(ctx context.Context, p domain.Payment) error
	List(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time, limit, offset int) ([]domain.Payment, error)
	Count(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time) (int, error)
}
