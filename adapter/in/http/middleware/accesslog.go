package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func AccessLog(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Info("http_request",
			zap.String("method", c.Method()),
			zap.String("route", c.Route().Path),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("latency", time.Since(start)),
			zap.String("request_id", RequestIDFrom(c)),
			zap.String("ip", c.IP()),
		)
		return err
	}
}
