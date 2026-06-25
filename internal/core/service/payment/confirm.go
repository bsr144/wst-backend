package payment

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
)

func (s *Service) Confirm(ctx context.Context, id uuid.UUID, input in.ConfirmPaymentInput) (domain.Payment, error) {
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
