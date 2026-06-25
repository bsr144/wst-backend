package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"wst-backend/internal/adapter/in/http/presenter"
)

func AccessLog() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			status = presenter.StatusCode(err)
		}
		presenter.LoggerFrom(c).Info("http_request",
			zap.String("method", c.Method()),
			zap.String("route", c.Route().Path),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.IP()),
		)
		return err
	}
}
