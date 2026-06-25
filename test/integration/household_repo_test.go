//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	pg "wst-backend/internal/adapter/out/postgres"
	"wst-backend/internal/core/domain"
)

func TestHouseholdRepository_DeleteRestrictsWithDependents(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
	households := pg.NewHouseholdRepository(testPool)

	err := households.Delete(ctx, hh.ID)
	require.ErrorIs(t, err, domain.ErrHouseholdHasDependents)

	_, ferr := households.FindByID(ctx, hh.ID)
	require.NoError(t, ferr)
}

func TestHouseholdRepository_DeleteRemovesChildlessHousehold(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	hh := seedHousehold(t)
	households := pg.NewHouseholdRepository(testPool)

	require.NoError(t, households.Delete(ctx, hh.ID))

	_, err := households.FindByID(ctx, hh.ID)
	require.ErrorIs(t, err, domain.ErrHouseholdNotFound)
}

func TestPickupRepository_InsertUnknownHousehold(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	pickups := pg.NewPickupRepository(testPool)

	orphan := domain.Pickup{ID: uuid.New(), HouseholdID: uuid.New(), Type: domain.PickupOrganic, Status: domain.PickupPending, CreatedAt: now, UpdatedAt: now}
	err := pickups.Insert(ctx, orphan)
	require.ErrorIs(t, err, domain.ErrHouseholdNotFound)
}
