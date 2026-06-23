package mock

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"wst-backend/core/domain"
	"wst-backend/core/port/out"
)

type ReportRepository struct {
	mock.Mock
}

func (m *ReportRepository) WasteSummary(ctx context.Context) ([]domain.WasteCount, error) {
	args := m.Called(ctx)
	var v []domain.WasteCount
	if r := args.Get(0); r != nil {
		v = r.([]domain.WasteCount)
	}
	return v, args.Error(1)
}

func (m *ReportRepository) PaymentSummary(ctx context.Context) ([]domain.PaymentStatusTotal, error) {
	args := m.Called(ctx)
	var v []domain.PaymentStatusTotal
	if r := args.Get(0); r != nil {
		v = r.([]domain.PaymentStatusTotal)
	}
	return v, args.Error(1)
}

func (m *ReportRepository) HouseholdPickups(ctx context.Context, householdID uuid.UUID) ([]domain.Pickup, error) {
	args := m.Called(ctx, householdID)
	var v []domain.Pickup
	if r := args.Get(0); r != nil {
		v = r.([]domain.Pickup)
	}
	return v, args.Error(1)
}

func (m *ReportRepository) HouseholdPayments(ctx context.Context, householdID uuid.UUID) ([]domain.Payment, error) {
	args := m.Called(ctx, householdID)
	var v []domain.Payment
	if r := args.Get(0); r != nil {
		v = r.([]domain.Payment)
	}
	return v, args.Error(1)
}

var _ out.ReportRepository = (*ReportRepository)(nil)
