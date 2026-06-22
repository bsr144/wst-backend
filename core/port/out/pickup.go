package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/core/domain"
)

type PickupRepository interface {
	Insert(ctx context.Context, p domain.Pickup) error
	List(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID, limit, offset int) ([]domain.Pickup, error)
	Count(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID) (int, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Pickup, error)
	Schedule(ctx context.Context, id uuid.UUID, pickupDate, now time.Time) (domain.Pickup, bool, error)
	Complete(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error)
	Cancel(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error)
}
