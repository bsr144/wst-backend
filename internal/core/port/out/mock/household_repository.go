package mock

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/out"
)

type HouseholdRepository struct {
	mock.Mock
}

func (m *HouseholdRepository) Insert(ctx context.Context, h domain.Household) error {
	args := m.Called(ctx, h)
	return args.Error(0)
}

func (m *HouseholdRepository) List(ctx context.Context, limit, offset int) ([]domain.Household, error) {
	args := m.Called(ctx, limit, offset)
	var items []domain.Household
	if v := args.Get(0); v != nil {
		items = v.([]domain.Household)
	}
	return items, args.Error(1)
}

func (m *HouseholdRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *HouseholdRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Household, error) {
	args := m.Called(ctx, id)
	var h domain.Household
	if v := args.Get(0); v != nil {
		h = v.(domain.Household)
	}
	return h, args.Error(1)
}

func (m *HouseholdRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

var (
	_ out.HouseholdRepository = (*HouseholdRepository)(nil)
	_ out.HouseholdReader     = (*HouseholdRepository)(nil)
	_ out.HouseholdWriter     = (*HouseholdRepository)(nil)
)
