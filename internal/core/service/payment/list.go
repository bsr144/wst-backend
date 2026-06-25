package payment

import (
	"context"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/pkg/pagination"
)

func (s *Service) List(ctx context.Context, filter in.PaymentFilter, params pagination.Params) ([]domain.Payment, int, error) {
	items, err := s.payments.List(ctx, filter.Status, filter.HouseholdID, filter.DateFrom, filter.DateTo, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, err
	}
	total, err := s.payments.Count(ctx, filter.Status, filter.HouseholdID, filter.DateFrom, filter.DateTo)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
