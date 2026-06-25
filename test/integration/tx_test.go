//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/adapter/out/clock"
	pg "wst-backend/internal/adapter/out/postgres"
	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/core/service/pickup"
)

func TestTxManager_RollsBackOnError(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	tx := pg.NewTxManager(testPool)
	households := pg.NewHouseholdRepository(testPool)
	id := uuid.New()
	wantErr := errors.New("boom")

	err := tx.Do(ctx, func(ctx context.Context) error {
		if e := households.Insert(ctx, domain.Household{ID: id, OwnerName: "o", Address: "a", CreatedAt: now, UpdatedAt: now}); e != nil {
			return e
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, ferr := households.FindByID(ctx, id)
	require.ErrorIs(t, ferr, domain.ErrHouseholdNotFound)

	var count int
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM households").Scan(&count))
	assert.Equal(t, 0, count)
}

func TestTxManager_Commits(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	tx := pg.NewTxManager(testPool)
	households := pg.NewHouseholdRepository(testPool)
	id := uuid.New()

	err := tx.Do(ctx, func(ctx context.Context) error {
		return households.Insert(ctx, domain.Household{ID: id, OwnerName: "o", Address: "a", CreatedAt: now, UpdatedAt: now})
	})
	require.NoError(t, err)

	got, ferr := households.FindByID(ctx, id)
	require.NoError(t, ferr)
	assert.Equal(t, id, got.ID)
}

func TestComplete_CreatesPaymentInSameTransaction(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	pickup := insertPickup(t, hh.ID, domain.PickupElectronic, domain.PickupScheduled, true, now)

	svc := newPickupService()
	payments := pg.NewPaymentRepository(testPool)

	completed, err := svc.Complete(ctx, pickup.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PickupCompleted, completed.Status)

	status := domain.PaymentPending
	list, err := payments.List(ctx, &status, &hh.ID, nil, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, pickup.ID, list[0].WasteID)
	assert.True(t, list[0].Amount.Equal(decimal.NewFromInt(100000)), "electronic price expected; got %s", list[0].Amount)
}

func newPickupService() *pickup.Service {
	pricing := domain.Pricing{Standard: decimal.NewFromInt(50000), Electronic: decimal.NewFromInt(100000)}
	return pickup.NewService(
		pg.NewPickupRepository(testPool),
		pg.NewPaymentRepository(testPool),
		pg.NewTxManager(testPool),
		clock.New(),
		pricing,
		72*time.Hour,
	)
}

func TestCreate_BlocksWhenHouseholdHasPendingPayment(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	existing := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	insertPayment(t, hh.ID, existing.ID, decimal.NewFromInt(10000), domain.PaymentPending, now)
	svc := newPickupService()

	_, err := svc.Create(ctx, in.CreatePickupCommand{HouseholdID: hh.ID, Type: domain.PickupOrganic})
	require.ErrorIs(t, err, domain.ErrHouseholdHasPendingPayment)

	count, err := pg.NewPickupRepository(testPool).Count(ctx, nil, &hh.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCreate_AllowedWhenOnlyPaidPayment(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	existing := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	insertPayment(t, hh.ID, existing.ID, decimal.NewFromInt(10000), domain.PaymentPaid, now)
	svc := newPickupService()

	created, err := svc.Create(ctx, in.CreatePickupCommand{HouseholdID: hh.ID, Type: domain.PickupPlastic})
	require.NoError(t, err)
	assert.Equal(t, domain.PickupPending, created.Status)
}
