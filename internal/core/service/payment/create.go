package payment

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
)

func (s *Service) Create(ctx context.Context, cmd in.CreatePaymentCommand) (domain.Payment, error) {
	pickup, err := s.pickups.FindByID(ctx, cmd.WasteID)
	if err != nil {
		return domain.Payment{}, err
	}
	if pickup.HouseholdID != cmd.HouseholdID {
		return domain.Payment{}, domain.ErrPaymentHouseholdMismatch
	}
	now := s.clock.Now()
	payment := domain.Payment{
		ID:          uuid.New(),
		HouseholdID: cmd.HouseholdID,
		WasteID:     cmd.WasteID,
		Amount:      s.pricing.AmountFor(pickup.Type),
		Status:      domain.PaymentPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.payments.Insert(ctx, payment); err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}
