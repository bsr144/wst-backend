package service

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	"wst-backend/core/port/out"
	"wst-backend/pkg/pagination"
)

type HouseholdService struct {
	repo  out.HouseholdRepository
	clock out.Clock
}

func NewHouseholdService(repo out.HouseholdRepository, clock out.Clock) *HouseholdService {
	return &HouseholdService{repo: repo, clock: clock}
}

func (s *HouseholdService) Create(ctx context.Context, cmd in.CreateHouseholdCommand) (domain.Household, error) {
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

func (s *HouseholdService) List(ctx context.Context, params pagination.Params) ([]domain.Household, int, error) {
	items, err := s.repo.List(ctx, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *HouseholdService) Get(ctx context.Context, id uuid.UUID) (domain.Household, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *HouseholdService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

var _ in.HouseholdService = (*HouseholdService)(nil)
