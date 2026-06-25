package handler

import (
	"github.com/gofiber/fiber/v2"

	"wst-backend/internal/adapter/in/http/dto"
	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/core/port/in"
)

type ReportHandler struct {
	svc in.ReportService
}

func NewReportHandler(svc in.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) WasteSummary(c *fiber.Ctx) error {
	summary, err := h.svc.WasteSummary(c.UserContext())
	if err != nil {
		return presenter.Error(c, err)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewWasteSummaryResponse(summary))
}

func (h *ReportHandler) PaymentSummary(c *fiber.Ctx) error {
	summary, err := h.svc.PaymentSummary(c.UserContext())
	if err != nil {
		return presenter.Error(c, err)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewPaymentSummaryResponse(summary))
}

func (h *ReportHandler) HouseholdHistory(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}
	history, svcErr := h.svc.HouseholdHistory(c.UserContext(), id)
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewHouseholdHistoryResponse(history))
}
