package report

import (
	"context"

	"github.com/shopspring/decimal"

	"wst-backend/internal/core/domain"
)

func (s *Service) PaymentSummary(ctx context.Context) (domain.PaymentSummary, error) {
	totals, err := s.reports.PaymentSummary(ctx)
	if err != nil {
		return domain.PaymentSummary{}, err
	}
	revenue := decimal.Zero
	for _, t := range totals {
		if t.Status == domain.PaymentPaid {
			revenue = revenue.Add(t.Amount)
		}
	}
	return domain.PaymentSummary{Totals: totals, TotalRevenue: revenue}, nil
}
