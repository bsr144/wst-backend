package handler

import (
	"io"
	"net/http"
	"strings"
	"time"

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

type UploadPolicy struct {
	MaxBytes     int64
	AllowedTypes []string
}

func (p UploadPolicy) allows(contentType string) bool {
	ct := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	for _, allowed := range p.AllowedTypes {
		if strings.EqualFold(allowed, ct) {
			return true
		}
	}
	return false
}

type PaymentHandler struct {
	svc    in.PaymentService
	upload UploadPolicy
}

func NewPaymentHandler(svc in.PaymentService, upload UploadPolicy) *PaymentHandler {
	return &PaymentHandler{svc: svc, upload: upload}
}

func (h *PaymentHandler) Create(c *fiber.Ctx) error {
	var req dto.CreatePaymentRequest
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
	wasteID, err := uuid.Parse(req.WasteID)
	if err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
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
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid query parameters"))
	}
	query.Sanitize()
	if err := validator.Struct(query); err != nil {
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

func (h *PaymentHandler) Confirm(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return presenter.Error(c, err)
	}

	fileHeader, err := c.FormFile("proof")
	if err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "proof", Reason: "a proof file is required"}))
	}
	if fileHeader.Size > h.upload.MaxBytes {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "proof", Reason: "file exceeds the maximum allowed size"}))
	}

	file, err := fileHeader.Open()
	if err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "proof", Reason: "could not read the uploaded file"}))
	}
	defer file.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "proof", Reason: "could not read the uploaded file"}))
	}
	contentType := http.DetectContentType(head[:n])
	if !h.upload.allows(contentType) {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "proof", Reason: "unsupported content type"}))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return presenter.Error(c, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "proof", Reason: "could not read the uploaded file"}))
	}

	payment, svcErr := h.svc.Confirm(c.UserContext(), id, in.ConfirmPaymentInput{
		Reader:      file,
		Size:        fileHeader.Size,
		ContentType: contentType,
	})
	if svcErr != nil {
		return presenter.Error(c, svcErr)
	}
	return presenter.Success(c, fiber.StatusOK, dto.NewPaymentResponse(payment))
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
			return in.PaymentFilter{}, apperr.Validation(apperr.CodeValidation, "invalid request").
				WithDetails(apperr.FieldError{Field: "household_id", Reason: "must be a valid uuid"})
		}
		filter.HouseholdID = &id
	}
	if query.DateFrom != "" {
		from, err := time.Parse(time.RFC3339, query.DateFrom)
		if err != nil {
			return in.PaymentFilter{}, apperr.Validation(apperr.CodeValidation, "invalid request").
				WithDetails(apperr.FieldError{Field: "date_from", Reason: "must be an RFC3339 timestamp"})
		}
		filter.DateFrom = &from
	}
	if query.DateTo != "" {
		to, err := time.Parse(time.RFC3339, query.DateTo)
		if err != nil {
			return in.PaymentFilter{}, apperr.Validation(apperr.CodeValidation, "invalid request").
				WithDetails(apperr.FieldError{Field: "date_to", Reason: "must be an RFC3339 timestamp"})
		}
		filter.DateTo = &to
	}
	if filter.DateFrom != nil && filter.DateTo != nil && filter.DateFrom.After(*filter.DateTo) {
		return in.PaymentFilter{}, apperr.Validation(apperr.CodeValidation, "invalid request").
			WithDetails(apperr.FieldError{Field: "date_from", Reason: "must not be after date_to"})
	}
	return filter, nil
}
