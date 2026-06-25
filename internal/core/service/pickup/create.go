package pickup

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
)

func (s *Service) Create(ctx context.Context, cmd in.CreatePickupCommand) (domain.Pickup, error) {
	now := s.clock.Now()
	pickup := domain.Pickup{
		ID:          uuid.New(),
		HouseholdID: cmd.HouseholdID,
		Type:        cmd.Type,
		Status:      domain.PickupPending,
		SafetyCheck: cmd.SafetyCheck,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		pending, err := s.payments.HasPendingByHousehold(ctx, cmd.HouseholdID)
		if err != nil {
			return err
		}
		if pending {
			return domain.ErrHouseholdHasPendingPayment
		}
		return s.pickups.Insert(ctx, pickup)
	})
	if err != nil {
		return domain.Pickup{}, err
	}
	return pickup, nil
}
