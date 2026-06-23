package in

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wst-backend/core/domain"
	"wst-backend/pkg/pagination"
)

type CreatePaymentCommand struct {
	HouseholdID uuid.UUID
	WasteID     uuid.UUID
}

type PaymentFilter struct {
	Status      *domain.PaymentStatus
	HouseholdID *uuid.UUID
	DateFrom    *time.Time
	DateTo      *time.Time
}

type PaymentService interface {
	Create(ctx context.Context, cmd CreatePaymentCommand) (domain.Payment, error)
	List(ctx context.Context, filter PaymentFilter, params pagination.Params) ([]domain.Payment, int, error)
}
