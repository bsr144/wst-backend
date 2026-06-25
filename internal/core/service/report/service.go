package report

import (
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/core/port/out"
)

type Service struct {
	reports    out.ReportReader
	households out.HouseholdReader
}

func NewService(reports out.ReportReader, households out.HouseholdReader) *Service {
	return &Service{reports: reports, households: households}
}

var _ in.ReportService = (*Service)(nil)
