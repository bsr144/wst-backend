package in

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/core/domain"
	"wst-backend/pkg/pagination"
)

type CreateHouseholdCommand struct {
	OwnerName string
	Address   string
}

type HouseholdService interface {
	Create(ctx context.Context, cmd CreateHouseholdCommand) (domain.Household, error)
	List(ctx context.Context, params pagination.Params) ([]domain.Household, int, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Household, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
