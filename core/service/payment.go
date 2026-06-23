package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	"wst-backend/core/port/out"
	"wst-backend/pkg/pagination"
)

type PaymentService struct {
	payments out.PaymentRepository
	pickups  out.PickupRepository
	storage  out.FileStorage
	clock    out.Clock
	pricing  domain.Pricing
}

func NewPaymentService(payments out.PaymentRepository, pickups out.PickupRepository, storage out.FileStorage, clock out.Clock, pricing domain.Pricing) *PaymentService {
	return &PaymentService{payments: payments, pickups: pickups, storage: storage, clock: clock, pricing: pricing}
}

func (s *PaymentService) Create(ctx context.Context, cmd in.CreatePaymentCommand) (domain.Payment, error) {
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

func (s *PaymentService) List(ctx context.Context, filter in.PaymentFilter, params pagination.Params) ([]domain.Payment, int, error) {
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

func (s *PaymentService) Confirm(ctx context.Context, id uuid.UUID, input in.ConfirmPaymentInput) (domain.Payment, error) {
	existing, err := s.payments.FindByID(ctx, id)
	if err != nil {
		return domain.Payment{}, err
	}
	if existing.Status != domain.PaymentPending {
		return domain.Payment{}, domain.ErrPaymentNotPending
	}

	key := fmt.Sprintf("payments/%s/%s%s", id, uuid.New(), extensionFor(input.ContentType))
	url, err := s.storage.Put(ctx, key, input.Reader, input.Size, input.ContentType)
	if err != nil {
		return domain.Payment{}, domain.ErrStorageUnavailable.WithCause(err)
	}

	paid, ok, err := s.payments.Confirm(ctx, id, url, s.clock.Now())
	if err != nil {
		return domain.Payment{}, err
	}
	if !ok {
		return domain.Payment{}, domain.ErrPaymentNotPending
	}
	return paid, nil
}

func extensionFor(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

var _ in.PaymentService = (*PaymentService)(nil)
