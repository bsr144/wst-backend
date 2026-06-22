package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"wst-backend/adapter/in/http/dto"
	"wst-backend/adapter/in/http/presenter"
	"wst-backend/core/port/in"
	"wst-backend/pkg/apperr"
	"wst-backend/pkg/pagination"
	"wst-backend/pkg/validatorx"
)

type HouseholdHandler struct {
	svc in.HouseholdService
}

func NewHouseholdHandler(svc in.HouseholdService) *HouseholdHandler {
	return &HouseholdHandler{svc: svc}
}

func (h *HouseholdHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateHouseholdRequest
	if err := c.BodyParser(&req); err != nil {
		return presenter.Error(c, apperr.Validation("VALIDATION_ERROR", "invalid request body"))
	}
	req.OwnerName = strings.TrimSpace(req.OwnerName)
	req.Address = strings.TrimSpace(req.Address)
	if err := validatorx.Struct(req); err != nil {
		return presenter.Error(c, err)
	}

	household, err := h.svc.Create(c.UserContext(), in.CreateHouseholdCommand{
		OwnerName: req.OwnerName,
		Address:   req.Address,
	})
	if err != nil {
		return presenter.Error(c, err)
	}
	return presenter.Success(c, fiber.StatusCreated, dto.NewHouseholdResponse(household))
}

func (h *HouseholdHandler) List(c *fiber.Ctx) error {
	params := pagination.Parse(c.Query("page"), c.Query("per_page"))
	items, total, err := h.svc.List(c.UserContext(), params)
	if err != nil {
		return presenter.Error(c, err)
	}
	return presenter.List(c, fiber.StatusOK, dto.NewHouseholdResponses(items), pagination.NewMeta(params, total))
}

func (h *HouseholdHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}
	household, svcErr := h.svc.Get(c.UserContext(), id)
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewHouseholdResponse(household))
}

func (h *HouseholdHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}
	if svcErr := h.svc.Delete(c.UserContext(), id); svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.NoContent(c)
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Validation("VALIDATION_ERROR", "invalid id").
			WithDetails(apperr.FieldError{Field: "id", Reason: "must be a valid uuid"})
	}
	return id, nil
}
