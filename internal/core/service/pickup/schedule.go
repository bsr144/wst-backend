package pickup

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
)

func (s *Service) Schedule(ctx context.Context, id uuid.UUID, cmd in.SchedulePickupCommand) (domain.Pickup, error) {
	pickup, ok, err := s.pickups.Schedule(ctx, id, cmd.PickupDate, s.clock.Now())
	if err != nil {
		return domain.Pickup{}, err
	}
	if !ok {
		current, ferr := s.pickups.FindByID(ctx, id)
		if ferr != nil {
			return domain.Pickup{}, ferr
		}
		if cerr := current.CanSchedule(); cerr != nil {
			return domain.Pickup{}, cerr
		}
		return domain.Pickup{}, domain.ErrPickupNotPending
	}
	return pickup, nil
}
