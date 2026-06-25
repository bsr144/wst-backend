package in

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/pkg/pagination"
)

type CreatePaymentCommand struct {
	HouseholdID uuid.UUID
	WasteID     uuid.UUID
}

type ConfirmPaymentInput struct {
	Reader      io.Reader
	Size        int64
	ContentType string
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
	Confirm(ctx context.Context, id uuid.UUID, input ConfirmPaymentInput) (domain.Payment, error)
}
