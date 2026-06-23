package mock

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"wst-backend/core/domain"
	"wst-backend/core/port/out"
)

type PickupRepository struct {
	mock.Mock
}

func (m *PickupRepository) Insert(ctx context.Context, p domain.Pickup) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *PickupRepository) List(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID, limit, offset int) ([]domain.Pickup, error) {
	args := m.Called(ctx, status, householdID, limit, offset)
	var items []domain.Pickup
	if v := args.Get(0); v != nil {
		items = v.([]domain.Pickup)
	}
	return items, args.Error(1)
}

func (m *PickupRepository) Count(ctx context.Context, status *domain.PickupStatus, householdID *uuid.UUID) (int, error) {
	args := m.Called(ctx, status, householdID)
	return args.Int(0), args.Error(1)
}

func (m *PickupRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Pickup, error) {
	args := m.Called(ctx, id)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Error(1)
}

func (m *PickupRepository) Schedule(ctx context.Context, id uuid.UUID, pickupDate, now time.Time) (domain.Pickup, bool, error) {
	args := m.Called(ctx, id, pickupDate, now)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Bool(1), args.Error(2)
}

func (m *PickupRepository) Complete(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error) {
	args := m.Called(ctx, id, now)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Bool(1), args.Error(2)
}

func (m *PickupRepository) Cancel(ctx context.Context, id uuid.UUID, now time.Time) (domain.Pickup, bool, error) {
	args := m.Called(ctx, id, now)
	var p domain.Pickup
	if v := args.Get(0); v != nil {
		p = v.(domain.Pickup)
	}
	return p, args.Bool(1), args.Error(2)
}

func (m *PickupRepository) CancelStaleOrganic(ctx context.Context, olderThan, now time.Time) (int, error) {
	args := m.Called(ctx, olderThan, now)
	return args.Int(0), args.Error(1)
}

var _ out.PickupRepository = (*PickupRepository)(nil)
