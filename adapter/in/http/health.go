package http

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"wst-backend/adapter/in/http/presenter"
	"wst-backend/pkg/apperr"
)

func registerHealth(app *fiber.App, ready func(context.Context) error) {
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/readyz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			return presenter.Error(c, apperr.Unavailable("SERVICE_UNAVAILABLE", "dependencies not ready"))
		}
		return c.SendStatus(fiber.StatusOK)
	})
}
