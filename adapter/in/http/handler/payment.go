package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"wst-backend/adapter/in/http/dto"
	"wst-backend/adapter/in/http/presenter"
	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	"wst-backend/pkg/apperr"
	"wst-backend/pkg/pagination"
	"wst-backend/pkg/validatorx"
)

type PaymentHandler struct {
	svc in.PaymentService
}

func NewPaymentHandler(svc in.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

func (h *PaymentHandler) Create(c *fiber.Ctx) error {
	var req dto.CreatePaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return presenter.Error(c, apperr.Validation("VALIDATION_ERROR", "invalid request body"))
	}
	req.HouseholdID = strings.TrimSpace(req.HouseholdID)
	req.WasteID = strings.TrimSpace(req.WasteID)
	if err := validatorx.Struct(req); err != nil {
		return presenter.Error(c, err)
	}

	householdID, err := uuid.Parse(req.HouseholdID)
	if err != nil {
		return presenter.Error(c, apperr.Validation("VALIDATION_ERROR", "invalid request").
			WithDetails(apperr.FieldError{Field: "household_id", Reason: "must be a valid uuid"}))
	}
	wasteID, err := uuid.Parse(req.WasteID)
	if err != nil {
		return presenter.Error(c, apperr.Validation("VALIDATION_ERROR", "invalid request").
			WithDetails(apperr.FieldError{Field: "waste_id", Reason: "must be a valid uuid"}))
	}

	payment, svcErr := h.svc.Create(c.UserContext(), in.CreatePaymentCommand{
		HouseholdID: householdID,
		WasteID:     wasteID,
	})
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusCreated, dto.NewPaymentResponse(payment))
}

func (h *PaymentHandler) List(c *fiber.Ctx) error {
	var query dto.ListPaymentsQuery
	if err := c.QueryParser(&query); err != nil {
		return presenter.Error(c, apperr.Validation("VALIDATION_ERROR", "invalid query parameters"))
	}
	query.Status = strings.TrimSpace(query.Status)
	query.HouseholdID = strings.TrimSpace(query.HouseholdID)
	query.DateFrom = strings.TrimSpace(query.DateFrom)
	query.DateTo = strings.TrimSpace(query.DateTo)
	if err := validatorx.Struct(query); err != nil {
		return presenter.Error(c, err)
	}

	filter, err := buildPaymentFilter(query)
	if err != nil {
		return presenter.Error(c, err)
	}

	params := pagination.Parse(c.Query("page"), c.Query("per_page"))
	items, total, svcErr := h.svc.List(c.UserContext(), filter, params)
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.List(c, fiber.StatusOK, dto.NewPaymentResponses(items), pagination.NewMeta(params, total))
}

func buildPaymentFilter(query dto.ListPaymentsQuery) (in.PaymentFilter, error) {
	var filter in.PaymentFilter
	if query.Status != "" {
		status := domain.PaymentStatus(query.Status)
		filter.Status = &status
	}
	if query.HouseholdID != "" {
		id, err := uuid.Parse(query.HouseholdID)
		if err != nil {
			return in.PaymentFilter{}, apperr.Validation("VALIDATION_ERROR", "invalid request").
				WithDetails(apperr.FieldError{Field: "household_id", Reason: "must be a valid uuid"})
		}
		filter.HouseholdID = &id
	}
	if query.DateFrom != "" {
		from, err := time.Parse(time.RFC3339, query.DateFrom)
		if err != nil {
			return in.PaymentFilter{}, apperr.Validation("VALIDATION_ERROR", "invalid request").
				WithDetails(apperr.FieldError{Field: "date_from", Reason: "must be an RFC3339 timestamp"})
		}
		filter.DateFrom = &from
	}
	if query.DateTo != "" {
		to, err := time.Parse(time.RFC3339, query.DateTo)
		if err != nil {
			return in.PaymentFilter{}, apperr.Validation("VALIDATION_ERROR", "invalid request").
				WithDetails(apperr.FieldError{Field: "date_to", Reason: "must be an RFC3339 timestamp"})
		}
		filter.DateTo = &to
	}
	if filter.DateFrom != nil && filter.DateTo != nil && filter.DateFrom.After(*filter.DateTo) {
		return in.PaymentFilter{}, apperr.Validation("VALIDATION_ERROR", "invalid request").
			WithDetails(apperr.FieldError{Field: "date_from", Reason: "must not be after date_to"})
	}
	return filter, nil
}
