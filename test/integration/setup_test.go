//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"wst-backend/db"
	pg "wst-backend/internal/adapter/out/postgres"
	"wst-backend/internal/core/domain"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("wst"),
		tcpostgres.WithUsername("wst"),
		tcpostgres.WithPassword("wst"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		os.Exit(1)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		os.Exit(1)
	}

	if err := db.Migrate(dsn, "up"); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pool:", err)
		os.Exit(1)
	}

	code := m.Run()

	testPool.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE payments, waste_pickups, households CASCADE")
	require.NoError(t, err)
}

func seedHousehold(t *testing.T) domain.Household {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hh := domain.Household{
		ID:        uuid.New(),
		OwnerName: "Test Owner",
		Address:   "1 Test Street",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, pg.NewHouseholdRepository(testPool).Insert(context.Background(), hh))
	return hh
}

func insertPickup(t *testing.T, householdID uuid.UUID, typ domain.PickupType, status domain.PickupStatus, safety bool, createdAt time.Time) domain.Pickup {
	t.Helper()
	p := domain.Pickup{
		ID:          uuid.New(),
		HouseholdID: householdID,
		Type:        typ,
		Status:      status,
		SafetyCheck: safety,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
	require.NoError(t, pg.NewPickupRepository(testPool).Insert(context.Background(), p))
	return p
}

func insertPayment(t *testing.T, householdID, wasteID uuid.UUID, amount decimal.Decimal, status domain.PaymentStatus, createdAt time.Time) domain.Payment {
	t.Helper()
	p := domain.Payment{
		ID:          uuid.New(),
		HouseholdID: householdID,
		WasteID:     wasteID,
		Amount:      amount,
		Status:      status,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
	require.NoError(t, pg.NewPaymentRepository(testPool).Insert(context.Background(), p))
	return p
}
