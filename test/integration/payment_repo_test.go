//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pg "wst-backend/internal/adapter/out/postgres"
	"wst-backend/internal/core/domain"
)

func TestPaymentRepository_DecimalRoundTrip(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	payments := pg.NewPaymentRepository(testPool)

	cases := []string{"12345.67", "99999999999.99", "0.10", "1000000.00"}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			amount := decimal.RequireFromString(c)
			pickup := insertPickup(t, hh.ID, domain.PickupElectronic, domain.PickupCompleted, true, now)
			p := insertPayment(t, hh.ID, pickup.ID, amount, domain.PaymentPending, now)

			got, err := payments.FindByID(ctx, p.ID)
			require.NoError(t, err)
			assert.True(t, got.Amount.Equal(amount), "round-trip changed amount: got %s want %s", got.Amount, amount)
			assert.Equal(t, c, got.Amount.StringFixed(2))
		})
	}
}

func TestPaymentRepository_ListDateRangeFilter(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	payments := pg.NewPaymentRepository(testPool)

	paidOn := func(d time.Time) {
		pickup := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
		date := d
		p := domain.Payment{ID: uuid.New(), HouseholdID: hh.ID, WasteID: pickup.ID, Amount: decimal.NewFromInt(10000), PaymentDate: &date, Status: domain.PaymentPaid, CreatedAt: now, UpdatedAt: now}
		require.NoError(t, payments.Insert(ctx, p))
	}
	feb := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	paidOn(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	paidOn(feb)
	paidOn(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	list, err := payments.List(ctx, nil, &hh.ID, &from, &to, 100, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].PaymentDate)
	assert.Equal(t, feb, list[0].PaymentDate.UTC())
}

func TestPaymentRepository_UniqueWasteID(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	pickup := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	payments := pg.NewPaymentRepository(testPool)

	first := domain.Payment{ID: uuid.New(), HouseholdID: hh.ID, WasteID: pickup.ID, Amount: decimal.NewFromInt(10000), Status: domain.PaymentPending, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, payments.Insert(ctx, first))

	second := domain.Payment{ID: uuid.New(), HouseholdID: hh.ID, WasteID: pickup.ID, Amount: decimal.NewFromInt(10000), Status: domain.PaymentPending, CreatedAt: now, UpdatedAt: now}
	err := payments.Insert(ctx, second)
	require.ErrorIs(t, err, domain.ErrPaymentAlreadyExists)
}

func TestPaymentRepository_Confirm(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	pickup := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	payments := pg.NewPaymentRepository(testPool)
	p := insertPayment(t, hh.ID, pickup.ID, decimal.NewFromInt(10000), domain.PaymentPending, now)

	got, ok, err := payments.Confirm(ctx, p.ID, "http://minio/proofs/proof.png", now)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, domain.PaymentPaid, got.Status)
	require.NotNil(t, got.ProofFileURL)
	assert.Equal(t, "http://minio/proofs/proof.png", *got.ProofFileURL)
	require.NotNil(t, got.PaymentDate)

	_, ok2, err := payments.Confirm(ctx, p.ID, "http://minio/proofs/other.png", now)
	require.NoError(t, err)
	assert.False(t, ok2)
}

func TestPaymentRepository_HasPendingByHousehold(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	pickup := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	payments := pg.NewPaymentRepository(testPool)

	pending, err := payments.HasPendingByHousehold(ctx, hh.ID)
	require.NoError(t, err)
	assert.False(t, pending)

	insertPayment(t, hh.ID, pickup.ID, decimal.NewFromInt(10000), domain.PaymentPending, now)
	pending, err = payments.HasPendingByHousehold(ctx, hh.ID)
	require.NoError(t, err)
	assert.True(t, pending)
}
