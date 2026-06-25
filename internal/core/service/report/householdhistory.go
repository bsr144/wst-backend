package report

import (
	"context"

	"github.com/google/uuid"

	"wst-backend/internal/core/domain"
)

func (s *Service) HouseholdHistory(ctx context.Context, householdID uuid.UUID) (domain.HouseholdHistory, error) {
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
