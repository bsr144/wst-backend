package mock

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"wst-backend/core/domain"
	"wst-backend/core/port/out"
)

type PaymentRepository struct {
	mock.Mock
}

func (m *PaymentRepository) HasPendingByHousehold(ctx context.Context, householdID uuid.UUID) (bool, error) {
	args := m.Called(ctx, householdID)
	return args.Bool(0), args.Error(1)
}

func (m *PaymentRepository) Insert(ctx context.Context, p domain.Payment) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

var _ out.PaymentRepository = (*PaymentRepository)(nil)
