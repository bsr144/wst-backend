package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	"wst-backend/core/port/out"
)

type ReportService struct {
	reports    out.ReportRepository
	households out.HouseholdRepository
}

func NewReportService(reports out.ReportRepository, households out.HouseholdRepository) *ReportService {
	return &ReportService{reports: reports, households: households}
}

func (s *ReportService) WasteSummary(ctx context.Context) (domain.WasteSummary, error) {
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

func (s *ReportService) PaymentSummary(ctx context.Context) (domain.PaymentSummary, error) {
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

func (s *ReportService) HouseholdHistory(ctx context.Context, householdID uuid.UUID) (domain.HouseholdHistory, error) {
	household, err := s.households.FindByID(ctx, householdID)
	if err != nil {
		return domain.HouseholdHistory{}, err
	}
	pickups, err := s.reports.HouseholdPickups(ctx, householdID)
	if err != nil {
		return domain.HouseholdHistory{}, err
	}
	payments, err := s.reports.HouseholdPayments(ctx, householdID)
	if err != nil {
		return domain.HouseholdHistory{}, err
	}
	return domain.HouseholdHistory{Household: household, Pickups: pickups, Payments: payments}, nil
}

var _ in.ReportService = (*ReportService)(nil)
