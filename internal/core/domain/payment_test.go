package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"wst-backend/internal/core/domain"
)

func TestPricing_AmountFor(t *testing.T) {
	t.Parallel()

	pricing := domain.Pricing{Standard: decimal.NewFromInt(50000), Electronic: decimal.NewFromInt(100000)}

	tests := []struct {
		typ  domain.PickupType
		want decimal.Decimal
	}{
		{domain.PickupOrganic, decimal.NewFromInt(50000)},
		{domain.PickupPlastic, decimal.NewFromInt(50000)},
		{domain.PickupPaper, decimal.NewFromInt(50000)},
		{domain.PickupElectronic, decimal.NewFromInt(100000)},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.typ), func(t *testing.T) {
			t.Parallel()
			assert.True(t, pricing.AmountFor(tc.typ).Equal(tc.want))
		})
	}
}
