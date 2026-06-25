package pickup

import (
	"time"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/core/port/out"
)

type Service struct {
	pickups    out.PickupRepository
	payments   out.PaymentRepository
	tx         out.TxManager
	clock      out.Clock
	pricing    domain.Pricing
	organicTTL time.Duration
}

func NewService(pickups out.PickupRepository, payments out.PaymentRepository, tx out.TxManager, clock out.Clock, pricing domain.Pricing, organicTTL time.Duration) *Service {
	return &Service{pickups: pickups, payments: payments, tx: tx, clock: clock, pricing: pricing, organicTTL: organicTTL}
}

var _ in.PickupService = (*Service)(nil)
