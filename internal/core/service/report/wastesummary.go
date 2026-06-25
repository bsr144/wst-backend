package report

import (
	"context"

	"wst-backend/internal/core/domain"
)

func (s *Service) WasteSummary(ctx context.Context) (domain.WasteSummary, error) {
	counts, err := s.reports.WasteSummary(ctx)
	if err != nil {
		return domain.WasteSummary{}, err
	}
	total := 0
	for _, c := range counts {
		total += c.Count
	}
	return domain.WasteSummary{Counts: counts, Total: total}, nil
}
