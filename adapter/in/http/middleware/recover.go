package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"wst-backend/adapter/in/http/presenter"
	"wst-backend/pkg/apperr"
)

func Recover(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic_recovered",
					zap.Any("panic", r),
					zap.String("request_id", RequestIDFrom(c)),
					zap.String("route", c.Route().Path),
				)
				err = presenter.Error(c, apperr.Internal("INTERNAL_ERROR", "internal server error"))
			}
		}()
		return c.Next()
	}
}
