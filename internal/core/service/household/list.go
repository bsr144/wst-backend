package household

import (
	"context"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/pkg/pagination"
)

func (s *Service) List(ctx context.Context, params pagination.Params) ([]domain.Household, int, error) {
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
