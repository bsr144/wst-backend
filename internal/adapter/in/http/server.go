package http

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.uber.org/zap"

	"wst-backend/internal/adapter/in/http/middleware"
	"wst-backend/internal/adapter/in/http/presenter"
	"wst-backend/internal/config"
)

type Server struct {
	app     *fiber.App
	api     fiber.Router
	cfg     config.Config
	limiter *middleware.IPRateLimiter
}

func NewServer(cfg config.Config, logger *zap.Logger, ready func(context.Context) error, limiter *middleware.IPRateLimiter) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "wst-backend",
		BodyLimit:             cfg.BodyLimitBytes,
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return presenter.Error(c, err)
		},
	})

	app.Use(middleware.RequestID(logger))
	app.Use(middleware.Recover())
	app.Use(middleware.AccessLog())
	app.Use(cors.New(cors.Config{AllowOrigins: cfg.CORSOrigins}))
	app.Use(middleware.Timeout(cfg.RequestTimeout))

	registerHealth(app, ready)

	return &Server{
		app:     app,
		api:     app.Group("/api"),
		cfg:     cfg,
		limiter: limiter,
	}
}

func (s *Server) App() *fiber.App { return s.app }

func (s *Server) API() fiber.Router { return s.api }

func (s *Server) PickupRateLimit() fiber.Handler { return s.limiter.Middleware() }

func (s *Server) Listen() error { return s.app.Listen(":" + s.cfg.AppPort) }

func (s *Server) Shutdown(ctx context.Context) error { return s.app.ShutdownWithContext(ctx) }
