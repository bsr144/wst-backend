package household

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
)

func (s *Service) Create(ctx context.Context, cmd in.CreateHouseholdCommand) (domain.Household, error) {
	now := s.clock.Now()
	h := domain.Household{
		ID:        uuid.New(),
		OwnerName: cmd.OwnerName,
		Address:   cmd.Address,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Insert(ctx, h); err != nil {
		return domain.Household{}, err
	}
	return h, nil
}
