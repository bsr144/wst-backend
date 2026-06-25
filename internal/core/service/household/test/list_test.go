package household_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/household"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
	"wst-backend/internal/pkg/pagination"
)

func TestHouseholdService_List(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	items := []domain.Household{
		{ID: uuid.New(), OwnerName: "Budi", Address: "Jl. Mawar 1", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), OwnerName: "Siti", Address: "Jl. Melati 2", CreatedAt: now, UpdatedAt: now},
	}
	repo := new(repomock.HouseholdRepository)
	svc := household.NewService(repo, servicetest.FixedClock{At: now})

	params := pagination.Params{Page: 2, PerPage: 10}
	repo.On("List", mock.Anything, 10, 10).Return(items, nil).Once()
	repo.On("Count", mock.Anything).Return(42, nil).Once()

	got, total, err := svc.List(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, 42, total)
	repo.AssertExpectations(t)
}

func TestHouseholdService_List_Errors(t *testing.T) {
	t.Parallel()

	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")

	t.Run("list error propagates and skips count", func(t *testing.T) {
		t.Parallel()
		repo := new(repomock.HouseholdRepository)
		svc := household.NewService(repo, servicetest.FixedClock{At: time.Now()})
		repo.On("List", mock.Anything, 20, 0).Return(nil, infraErr).Once()

		items, total, err := svc.List(context.Background(), pagination.Params{Page: 1, PerPage: 20})

		require.ErrorIs(t, err, infraErr)
		assert.Nil(t, items)
		assert.Zero(t, total)
		repo.AssertNotCalled(t, "Count", mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("count error propagates after successful list", func(t *testing.T) {
		t.Parallel()
		repo := new(repomock.HouseholdRepository)
		svc := household.NewService(repo, servicetest.FixedClock{At: time.Now()})
		repo.On("List", mock.Anything, 20, 0).Return([]domain.Household{}, nil).Once()
		repo.On("Count", mock.Anything).Return(0, infraErr).Once()

		items, total, err := svc.List(context.Background(), pagination.Params{Page: 1, PerPage: 20})

		require.ErrorIs(t, err, infraErr)
		assert.Nil(t, items)
		assert.Zero(t, total)
		repo.AssertExpectations(t)
	})
}
