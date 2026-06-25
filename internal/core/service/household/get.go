package household

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Household, error) {
	return s.repo.FindByID(ctx, id)
}
