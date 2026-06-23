package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	"wst-backend/core/port/out"
	"wst-backend/pkg/pagination"
)

type PickupService struct {
	pickups    out.PickupRepository
	payments   out.PaymentRepository
	tx         out.TxManager
	clock      out.Clock
	pricing    domain.Pricing
	organicTTL time.Duration
}

func NewPickupService(pickups out.PickupRepository, payments out.PaymentRepository, tx out.TxManager, clock out.Clock, pricing domain.Pricing, organicTTL time.Duration) *PickupService {
	return &PickupService{pickups: pickups, payments: payments, tx: tx, clock: clock, pricing: pricing, organicTTL: organicTTL}
}

func (s *PickupService) Create(ctx context.Context, cmd in.CreatePickupCommand) (domain.Pickup, error) {
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

func (s *PickupService) List(ctx context.Context, filter in.PickupFilter, params pagination.Params) ([]domain.Pickup, int, error) {
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

func (s *PickupService) Schedule(ctx context.Context, id uuid.UUID, cmd in.SchedulePickupCommand) (domain.Pickup, error) {
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

func (s *PickupService) Complete(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
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

func (s *PickupService) Cancel(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
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

func (s *PickupService) CancelStaleOrganic(ctx context.Context) (int, error) {
	now := s.clock.Now()
	cutoff := now.Add(-s.organicTTL)
	return s.pickups.CancelStaleOrganic(ctx, cutoff, now)
}

var _ in.PickupService = (*PickupService)(nil)
