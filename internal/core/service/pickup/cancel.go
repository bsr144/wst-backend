package pickup

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
	pickup, ok, err := s.pickups.Cancel(ctx, id, s.clock.Now())
	if err != nil {
		return domain.Pickup{}, err
	}
	if !ok {
		if _, ferr := s.pickups.FindByID(ctx, id); ferr != nil {
			return domain.Pickup{}, ferr
		}
		return domain.Pickup{}, domain.ErrPickupNotCancelable
	}
	return pickup, nil
}
