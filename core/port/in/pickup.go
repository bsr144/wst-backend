package in

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/core/domain"
	"wst-backend/pkg/pagination"
)

type CreatePickupCommand struct {
	HouseholdID uuid.UUID
	Type        domain.PickupType
	SafetyCheck bool
}

type PickupFilter struct {
	Status      *domain.PickupStatus
	HouseholdID *uuid.UUID
}

type SchedulePickupCommand struct {
	PickupDate time.Time
}

type PickupService interface {
	Create(ctx context.Context, cmd CreatePickupCommand) (domain.Pickup, error)
	List(ctx context.Context, filter PickupFilter, params pagination.Params) ([]domain.Pickup, int, error)
	Schedule(ctx context.Context, id uuid.UUID, cmd SchedulePickupCommand) (domain.Pickup, error)
	Complete(ctx context.Context, id uuid.UUID) (domain.Pickup, error)
	Cancel(ctx context.Context, id uuid.UUID) (domain.Pickup, error)
}
