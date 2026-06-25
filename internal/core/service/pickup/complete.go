package pickup

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

func (s *Service) Complete(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
	now := s.clock.Now()
	var completed domain.Pickup
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		pickup, ok, err := s.pickups.Complete(ctx, id, now)
		if err != nil {
			return err
		}
		if !ok {
			if _, ferr := s.pickups.FindByID(ctx, id); ferr != nil {
				return ferr
			}
			return domain.ErrPickupNotScheduled
		}
		payment := domain.Payment{
			ID:          uuid.New(),
			HouseholdID: pickup.HouseholdID,
			WasteID:     pickup.ID,
			Amount:      s.pricing.AmountFor(pickup.Type),
			Status:      domain.PaymentPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.payments.Insert(ctx, payment); err != nil {
			return err
		}
		completed = pickup
		return nil
	})
	if err != nil {
		return domain.Pickup{}, err
	}
	return completed, nil
}
