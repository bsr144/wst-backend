package http

import (
	"github.com/gofiber/fiber/v2"

	"wst-backend/adapter/in/http/handler"
)

func RegisterRoutes(api fiber.Router, h *handler.HouseholdHandler) {
	households := api.Group("/households")
	households.Post("/", h.Create)
	households.Get("/", h.List)
	households.Get("/:id", h.Get)
	households.Delete("/:id", h.Delete)
}

func RegisterPickupRoutes(api fiber.Router, h *handler.PickupHandler, rateLimit fiber.Handler) {
	pickups := api.Group("/pickups")
	pickups.Post("/", rateLimit, h.Create)
	pickups.Get("/", h.List)
	pickups.Put("/:id/schedule", h.Schedule)
	pickups.Put("/:id/complete", h.Complete)
	pickups.Put("/:id/cancel", h.Cancel)
}

func RegisterPaymentRoutes(api fiber.Router, h *handler.PaymentHandler) {
	payments := api.Group("/payments")
	payments.Post("/", h.Create)
	payments.Get("/", h.List)
}
