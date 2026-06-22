package out

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/core/domain"
)

type PaymentRepository interface {
	HasPendingByHousehold(ctx context.Context, householdID uuid.UUID) (bool, error)
	Insert(ctx context.Context, p domain.Payment) error
}
