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
	"wst-backend/internal/core/port/in"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/household"
	"wst-backend/internal/core/service/servicetest"
	"wst-backend/internal/pkg/apperr"
)

func TestHouseholdService_Create(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	repo := new(repomock.HouseholdRepository)
	svc := household.NewService(repo, servicetest.FixedClock{At: now})

	repo.On("Insert", mock.Anything, mock.MatchedBy(func(h domain.Household) bool {
		return h.OwnerName == "Budi" &&
			h.Address == "Jl. Mawar 1" &&
			h.CreatedAt.Equal(now) &&
			h.UpdatedAt.Equal(now) &&
			h.ID != uuid.Nil
	})).Return(nil).Once()

	got, err := svc.Create(context.Background(), in.CreateHouseholdCommand{OwnerName: "Budi", Address: "Jl. Mawar 1"})

	require.NoError(t, err)
	assert.Equal(t, "Budi", got.OwnerName)
	assert.Equal(t, "Jl. Mawar 1", got.Address)
	assert.Equal(t, now, got.CreatedAt)
	assert.Equal(t, now, got.UpdatedAt)
	assert.NotEqual(t, uuid.Nil, got.ID)
	repo.AssertExpectations(t)
}

func TestHouseholdService_Create_InsertError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	infraErr := apperr.Unavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	repo := new(repomock.HouseholdRepository)
	svc := household.NewService(repo, servicetest.FixedClock{At: now})

	repo.On("Insert", mock.Anything, mock.Anything).Return(infraErr).Once()

	got, err := svc.Create(context.Background(), in.CreateHouseholdCommand{OwnerName: "Budi", Address: "Jl. Mawar 1"})

	require.ErrorIs(t, err, infraErr)
	assert.Equal(t, domain.Household{}, got)
	repo.AssertExpectations(t)
}
