package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/pkg/apperr"
)

func Recover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				presenter.LoggerFrom(c).Error("panic_recovered",
					zap.Any("panic", r),
					zap.String("route", c.Route().Path),
					zap.Stack("stack"),
				)
				err = presenter.Error(c, apperr.Internal(apperr.CodeInternal, "recovered panic"))
			}
		}()
		return c.Next()
	}
}
