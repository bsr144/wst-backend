package out

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/core/domain"
)

type HouseholdRepository interface {
	Insert(ctx context.Context, h domain.Household) error
	List(ctx context.Context, limit, offset int) ([]domain.Household, error)
	Count(ctx context.Context) (int, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Household, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
