package servicetest

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"wst-backend/internal/core/domain"
)

type FixedClock struct {
	At time.Time
}

func (c FixedClock) Now() time.Time { return c.At }

type PassthroughTx struct{}

func (PassthroughTx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

var Pricing = domain.Pricing{Standard: decimal.NewFromInt(50000), Electronic: decimal.NewFromInt(100000)}

const OrganicTTL = 72 * time.Hour
