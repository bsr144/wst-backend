package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

type PickupReader interface {
	List(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID, limit, offset int) ([]domain.Pickup, error)
	Count(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID) (int, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Pickup, error)
}

type PickupWriter interface {
	Insert(ctx context.Context, p domain.Pickup) error
	Schedule(ctx context.Context, id uuid.UUID, pickupDate, now time.Time) (domain.Pickup, bool, error)
	Complete(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error)
	Cancel(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error)
	CancelStaleOrganic(ctx context.Context, olderThan, now time.Time) (int, error)
}

type PickupRepository interface {
	PickupReader
	PickupWriter
}
