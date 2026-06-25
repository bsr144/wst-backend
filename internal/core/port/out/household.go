package out

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

type HouseholdReader interface {
	List(ctx context.Context, limit, offset int) ([]domain.Household, error)
	Count(ctx context.Context) (int, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Household, error)
}

type HouseholdWriter interface {
	Insert(ctx context.Context, h domain.Household) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type HouseholdRepository interface {
	HouseholdReader
	HouseholdWriter
}
