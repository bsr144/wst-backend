package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"wst-backend/internal/adapter/in/http/dto"
	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/core/domain"
	"wst-backend/internal/core/port/in"
	"wst-backend/internal/pkg/apperr"
	"wst-backend/internal/pkg/pagination"
	"wst-backend/internal/pkg/validator"
)

type PickupHandler struct {
	svc in.PickupService
}

func NewPickupHandler(svc in.PickupService) *PickupHandler {
	return &PickupHandler{svc: svc}
}

func (h *PickupHandler) Create(c *fiber.Ctx) error {
	var req dto.CreatePickupRequest
	if err := c.BodyParser(&req); err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request body"))
	}
	req.Sanitize()
	if err := validator.Struct(req); err != nil {
		return presenter.Error(c, err)
	}

	householdID, err := uuid.Parse(req.HouseholdID)
	if err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "household_id", Reason: "must be a valid uuid"}))
	}

	pickup, svcErr := h.svc.Create(c.UserContext(), in.CreatePickupCommand{
		HouseholdID: householdID,
		Type:        domain.PickupType(req.Type),
		SafetyCheck: req.SafetyCheck != nil && *req.SafetyCheck,
	})
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusCreated, dto.NewPickupResponse(pickup))
}

func (h *PickupHandler) List(c *fiber.Ctx) error {
	var query dto.ListPickupsQuery
	if err := c.QueryParser(&query); err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid query parameters"))
	}
	query.Sanitize()
	if err := validator.Struct(query); err != nil {
		return presenter.Error(c, err)
	}

	filter, err := buildPickupFilter(query)
	if err != nil {
		return presenter.Error(c, err)
	}

	params := pagination.Parse(c.Query("page"), c.Query("per_page"))
	items, total, svcErr := h.svc.List(c.UserContext(), filter, params)
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.List(c, fiber.StatusOK, dto.NewPickupResponses(items), pagination.NewMeta(params, total))
}

func (h *PickupHandler) Schedule(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}
	var req dto.SchedulePickupRequest
	if err := c.BodyParser(&req); err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request body"))
	}
	if err := validator.Struct(req); err != nil {
		return presenter.Error(c, err)
	}

	pickup, svcErr := h.svc.Schedule(c.UserContext(), id, in.SchedulePickupCommand{PickupDate: *req.PickupDate})
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewPickupResponse(pickup))
}

func (h *PickupHandler) Complete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}
	pickup, svcErr := h.svc.Complete(c.UserContext(), id)
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewPickupResponse(pickup))
}

func (h *PickupHandler) Cancel(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}
	pickup, svcErr := h.svc.Cancel(c.UserContext(), id)
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewPickupResponse(pickup))
}

func buildPickupFilter(query dto.ListPickupsQuery) (in.PickupFilter, error) {
	var filter in.PickupFilter
	if query.Status != "" {
		status := domain.PickupStatus(query.Status)
		filter.Status = &status
	}
	if query.HouseholdID != "" {
		id, err := uuid.Parse(query.HouseholdID)
		if err != nil {
			return in.PickupFilter{}, apperr.Validation(apperr.CodeValidation, "invalid request").
				WithDetails(apperr.FieldError{Field: "household_id", Reason: "must be a valid uuid"})
		}
		filter.HouseholdID = &id
	}
	return filter, nil
}
