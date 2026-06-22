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
