package mock

import (
	"context"
	"time"

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

func (m *PaymentRepository) List(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time, limit, offset int) ([]domain.Payment, error) {
	args := m.Called(ctx, status, householdID, dateFrom, dateTo, limit, offset)
	var items []domain.Payment
	if v := args.Get(0); v != nil {
		items = v.([]domain.Payment)
	}
	return items, args.Error(1)
}

func (m *PaymentRepository) Count(ctx context.Context, status *domain.PaymentStatus, householdID *uuid.UUID, dateFrom, dateTo *time.Time) (int, error) {
	args := m.Called(ctx, status, householdID, dateFrom, dateTo)
	return args.Int(0), args.Error(1)
}

var _ out.PaymentRepository = (*PaymentRepository)(nil)
