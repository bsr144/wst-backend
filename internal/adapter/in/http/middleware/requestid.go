package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"wst-backend/internal/adapter/in/http/presenter"
)

const requestIDKey = "request_id"

const headerRequestID = "X-Request-ID"

func RequestID(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(headerRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(requestIDKey, id)
		c.Set(headerRequestID, id)
		presenter.WithLogger(c, logger.With(zap.String(requestIDKey, id)))
		return c.Next()
	}
}

func RequestIDFrom(c *fiber.Ctx) string {
	id, _ := c.Locals(requestIDKey).(string)
	return id
}
