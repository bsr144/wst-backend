package presenter

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"wst-backend/pkg/apperr"
)

type successEnvelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Details []apperr.FieldError `json:"details,omitempty"`
}

func Success(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(successEnvelope{Data: data})
}

func List(c *fiber.Ctx, status int, data any, meta any) error {
	return c.Status(status).JSON(successEnvelope{Data: data, Meta: meta})
}

func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

func Error(c *fiber.Ctx, err error) error {
	if ae, ok := apperr.From(err); ok {
		if ae.Kind == apperr.KindInternal {
			return write(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		}
		return write(c, statusFor(ae.Kind), ae.Code, ae.Message, ae.Details)
	}

	var fe *fiber.Error
	if errors.As(err, &fe) {
		return write(c, fe.Code, codeForStatus(fe.Code), fe.Message, nil)
	}

	return write(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
}

func write(c *fiber.Ctx, status int, code, message string, details []apperr.FieldError) error {
	return c.Status(status).JSON(errorEnvelope{Error: errorBody{Code: code, Message: message, Details: details}})
}

func statusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.KindValidation:
		return fiber.StatusBadRequest
	case apperr.KindNotFound:
		return fiber.StatusNotFound
	case apperr.KindConflict:
		return fiber.StatusConflict
	case apperr.KindUnprocessable:
		return fiber.StatusUnprocessableEntity
	case apperr.KindUnavailable:
		return fiber.StatusServiceUnavailable
	case apperr.KindRateLimited:
		return fiber.StatusTooManyRequests
	default:
		return fiber.StatusInternalServerError
	}
}

func codeForStatus(status int) string {
	switch status {
	case fiber.StatusNotFound:
		return "NOT_FOUND"
	case fiber.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	default:
		return "REQUEST_ERROR"
	}
}
