package pickup

import (
	"context"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/pkg/pagination"
)

func (s *Service) List(ctx context.Context, filter in.PickupFilter, params pagination.Params) ([]domain.Pickup, int, error) {
	items, err := s.pickups.List(ctx, filter.Status, filter.HouseholdID, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, err
	}
	total, err := s.pickups.Count(ctx, filter.Status, filter.HouseholdID)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
