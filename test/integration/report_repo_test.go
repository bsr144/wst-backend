//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pg "wst-backend/internal/adapter/out/postgres"
	"wst-backend/internal/core/domain"
)

func TestReportRepository_WasteSummary(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)

	insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
	insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupPending, false, now)
	insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	insertPickup(t, hh.ID, domain.PickupElectronic, domain.PickupScheduled, true, now)

	counts, err := pg.NewReportRepository(testPool).WasteSummary(ctx)
	require.NoError(t, err)

	got := map[string]int{}
	for _, c := range counts {
		got[string(c.Type)+"/"+string(c.Status)] = c.Count
	}
	assert.Equal(t, 2, got["organic/pending"])
	assert.Equal(t, 1, got["organic/completed"])
	assert.Equal(t, 1, got["electronic/scheduled"])
}

func TestReportRepository_PaymentSummary(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)

	p1 := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	p2 := insertPickup(t, hh.ID, domain.PickupPlastic, domain.PickupCompleted, false, now)
	p3 := insertPickup(t, hh.ID, domain.PickupPaper, domain.PickupCompleted, false, now)
	insertPayment(t, hh.ID, p1.ID, decimal.NewFromInt(50000), domain.PaymentPaid, now)
	insertPayment(t, hh.ID, p2.ID, decimal.NewFromInt(20000), domain.PaymentPaid, now)
	insertPayment(t, hh.ID, p3.ID, decimal.NewFromInt(10000), domain.PaymentPending, now)

	totals, err := pg.NewReportRepository(testPool).PaymentSummary(ctx)
	require.NoError(t, err)

	byStatus := map[domain.PaymentStatus]domain.PaymentStatusTotal{}
	for _, st := range totals {
		byStatus[st.Status] = st
	}
	assert.Equal(t, 2, byStatus[domain.PaymentPaid].Count)
	assert.True(t, byStatus[domain.PaymentPaid].Amount.Equal(decimal.NewFromInt(70000)), "paid revenue; got %s", byStatus[domain.PaymentPaid].Amount)
	assert.Equal(t, 1, byStatus[domain.PaymentPending].Count)
}

func TestReportRepository_HouseholdHistory(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := seedHousehold(t)
	other := seedHousehold(t)

	p1 := insertPickup(t, hh.ID, domain.PickupOrganic, domain.PickupCompleted, false, now)
	insertPickup(t, hh.ID, domain.PickupPlastic, domain.PickupPending, false, now)
	insertPickup(t, other.ID, domain.PickupPaper, domain.PickupPending, false, now)
	insertPayment(t, hh.ID, p1.ID, decimal.NewFromInt(10000), domain.PaymentPaid, now)

	reports := pg.NewReportRepository(testPool)

	pickups, err := reports.HouseholdPickups(ctx, hh.ID)
	require.NoError(t, err)
	assert.Len(t, pickups, 2)

	payments, err := reports.HouseholdPayments(ctx, hh.ID)
	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.True(t, payments[0].Amount.Equal(decimal.NewFromInt(10000)))
}
