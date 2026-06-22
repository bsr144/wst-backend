package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"wst-backend/core/domain"
)

func TestPricing_AmountFor(t *testing.T) {
	t.Parallel()

	pricing := domain.Pricing{Standard: decimal.NewFromInt(10000), Electronic: decimal.NewFromInt(50000)}

	tests := []struct {
		typ  domain.PickupType
		want decimal.Decimal
	}{
		{domain.PickupOrganic, decimal.NewFromInt(10000)},
		{domain.PickupPlastic, decimal.NewFromInt(10000)},
		{domain.PickupPaper, decimal.NewFromInt(10000)},
		{domain.PickupElectronic, decimal.NewFromInt(50000)},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.typ), func(t *testing.T) {
			t.Parallel()
			assert.True(t, pricing.AmountFor(tc.typ).Equal(tc.want))
		})
	}
}
