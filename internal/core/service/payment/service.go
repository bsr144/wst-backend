package payment

import (
	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/core/port/out"
)

type Service struct {
	payments out.PaymentRepository
	pickups  out.PickupReader
	storage  out.FileStorage
	clock    out.Clock
	pricing  domain.Pricing
}

func NewService(payments out.PaymentRepository, pickups out.PickupReader, storage out.FileStorage, clock out.Clock, pricing domain.Pricing) *Service {
	return &Service{payments: payments, pickups: pickups, storage: storage, clock: clock, pricing: pricing}
}

var _ in.PaymentService = (*Service)(nil)
