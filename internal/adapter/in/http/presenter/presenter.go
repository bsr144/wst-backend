package presenter

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"wst-backend/internal/pkg/apperr"
)

const loggerLocalsKey = "request_logger"

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

func WithLogger(c *fiber.Ctx, logger *zap.Logger) {
	c.Locals(loggerLocalsKey, logger)
}

func LoggerFrom(c *fiber.Ctx) *zap.Logger {
	if logger, ok := c.Locals(loggerLocalsKey).(*zap.Logger); ok && logger != nil {
		return logger
	}
	return zap.NewNop()
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
		return writeAppError(c, ae)
	}

	var fe *fiber.Error
	if errors.As(err, &fe) {
		code := codeForStatus(fe.Code)
		logRejected(c, fe.Code, code, err, nil)
		return write(c, fe.Code, code, fe.Message, nil)
	}

	return writeAppError(c, apperr.Internal(apperr.CodeInternal, "unhandled error").WithCause(err))
}

func writeAppError(c *fiber.Ctx, ae *apperr.Error) error {
	status := ae.Status()
	if ae.IsServerError() {
		logFailed(c, status, ae)
		return write(c, status, ae.ClientCode(), ae.ClientMessage(), nil)
	}
	logRejected(c, status, ae.Code, ae.Cause(), ae.Details)
	return write(c, status, ae.Code, ae.Message, ae.Details)
}

func StatusCode(err error) int {
	if ae, ok := apperr.From(err); ok {
		return ae.Status()
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return fe.Code
	}
	return fiber.StatusInternalServerError
}

func write(c *fiber.Ctx, status int, code, message string, details []apperr.FieldError) error {
	return c.Status(status).JSON(errorEnvelope{Error: errorBody{Code: code, Message: message, Details: details}})
}

func logFailed(c *fiber.Ctx, status int, ae *apperr.Error) {
	fields := []zap.Field{
		zap.Int("status", status),
		zap.String("code", ae.Code),
		zap.String("method", c.Method()),
		zap.String("route", c.Route().Path),
	}
	if cause := ae.Cause(); cause != nil {
		fields = append(fields, zap.Error(cause))
	} else {
		fields = append(fields, zap.String("error", ae.Error()))
	}
	LoggerFrom(c).Error("request_failed", fields...)
}

func logRejected(c *fiber.Ctx, status int, code string, cause error, details []apperr.FieldError) {
	fields := []zap.Field{
		zap.Int("status", status),
		zap.String("code", code),
		zap.String("method", c.Method()),
		zap.String("route", c.Route().Path),
	}
	if cause != nil {
		fields = append(fields, zap.Error(cause))
	}
	if len(details) > 0 {
		fields = append(fields, zap.Any("details", details))
	}
	LoggerFrom(c).Warn("request_rejected", fields...)
}

func codeForStatus(status int) string {
	switch status {
	case fiber.StatusNotFound:
		return apperr.CodeNotFound
	case fiber.StatusMethodNotAllowed:
		return apperr.CodeMethodNotAllowed
	default:
		return apperr.CodeRequestError
	}
}
