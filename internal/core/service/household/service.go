package household

import (
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/core/port/out"
)

type Service struct {
	repo  out.HouseholdRepository
	clock out.Clock
}

func NewService(repo out.HouseholdRepository, clock out.Clock) *Service {
	return &Service{repo: repo, clock: clock}
}

var _ in.HouseholdService = (*Service)(nil)
