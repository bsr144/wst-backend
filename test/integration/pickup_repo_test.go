//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pg "wst-backend/internal/adapter/out/postgres"
	"wst-backend/internal/core/domain"
)

func TestPickupRepository_GuardedTransitions(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	pickups := pg.NewPickupRepository(testPool)

	t.Run("schedule moves pending to scheduled", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
		got, ok, err := pickups.Schedule(ctx, p.ID, now.Add(24*time.Hour), now)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, domain.PickupScheduled, got.Status)
		require.NotNil(t, got.PickupDate)
	})

	t.Run("schedule fails when not pending", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
		_, ok, err := pickups.Schedule(ctx, p.ID, now.Add(time.Hour), now)
		require.NoError(t, err)
		require.True(t, ok)
		_, ok2, err := pickups.Schedule(ctx, p.ID, now.Add(time.Hour), now)
		require.NoError(t, err)
		assert.False(t, ok2)
	})

	t.Run("electronic without safety check cannot schedule", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupElectronic, domain.PickupPending, false, now)
		_, ok, err := pickups.Schedule(ctx, p.ID, now.Add(time.Hour), now)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("electronic with safety check can schedule", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupElectronic, domain.PickupPending, true, now)
		_, ok, err := pickups.Schedule(ctx, p.ID, now.Add(time.Hour), now)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("complete moves scheduled to completed", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupScheduled, false, now)
		got, ok, err := pickups.Complete(ctx, p.ID, now)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, domain.PickupCompleted, got.Status)
	})

	t.Run("complete fails when not scheduled", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
		_, ok, err := pickups.Complete(ctx, p.ID, now)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("cancel moves pending to canceled", func(t *testing.T) {
		p := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
		got, ok, err := pickups.Cancel(ctx, p.ID, now)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, domain.PickupCanceled, got.Status)
	})
}

func TestPickupRepository_CancelStaleOrganic(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := now.Add(-100 * time.Hour)
	cutoff := now.Add(-72 * time.Hour)
	hh := seedHousehold(t)
	pickups := pg.NewPickupRepository(testPool)

	staleOrganic := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, old)
	staleScheduledOrganic := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupScheduled, false, old)
	freshOrganic := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
	staleElectronic := insertPickup(t, hh.ID, domain.PickupElectronic, domain.PickupPending, false, old)
	staleCompletedOrganic := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, old)

	n, err := pickups.CancelStaleOrganic(ctx, cutoff, now)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	assertStatus(t, pickups, staleOrganic.ID, domain.PickupCanceled)
	assertStatus(t, pickups, staleScheduledOrganic.ID, domain.PickupCanceled)
	assertStatus(t, pickups, freshOrganic.ID, domain.PickupPending)
	assertStatus(t, pickups, staleElectronic.ID, domain.PickupPending)
	assertStatus(t, pickups, staleCompletedOrganic.ID, domain.PickupCompleted)
}

func assertStatus(t *testing.T, pickups *pg.PickupRepository, id uuid.UUID, want domain.PickupStatus) {
	t.Helper()
	got, err := pickups.FindByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, want, got.Status)
}
