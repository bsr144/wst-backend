package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	httpx "wst-backend/adapter/in/http"
	"wst-backend/adapter/in/http/handler"
	"wst-backend/adapter/in/http/middleware"
	"wst-backend/adapter/in/worker"
	"wst-backend/adapter/out/clock"
	"wst-backend/adapter/out/postgres"
	"wst-backend/adapter/out/storage"
	"wst-backend/config"
	"wst-backend/core/domain"
	"wst-backend/core/port/out"
	"wst-backend/core/service"
)

type App struct {
	cfg       config.Config
	logger    *zap.Logger
	pool      *pgxpool.Pool
	storage   *storage.MinIO
	server    *httpx.Server
	scheduler *worker.Scheduler
	limiter   *middleware.IPRateLimiter
	txManager out.TxManager
	clock     out.Clock
}

func New(ctx context.Context, cfg config.Config, logger *zap.Logger) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	store, err := storage.NewMinIO(ctx, cfg.MinIO)
	if err != nil {
		pool.Close()
		return nil, err
	}

	txManager := postgres.NewTxManager(pool)
	clk := clock.New()
	limiter := middleware.NewIPRateLimiter(cfg.RateLimit.RPM, cfg.RateLimit.Burst)

	ready := func(c context.Context) error {
		if err := pool.Ping(c); err != nil {
			return err
		}
		return store.Ping(c)
	}

	server := httpx.NewServer(cfg, logger, ready, limiter)
	scheduler := worker.New(logger)

	householdRepo := postgres.NewHouseholdRepository(pool)
	householdService := service.NewHouseholdService(householdRepo, clk)
	householdHandler := handler.NewHouseholdHandler(householdService)
	httpx.RegisterRoutes(server.API(), householdHandler)

	pickupRepo := postgres.NewPickupRepository(pool)
	paymentRepo := postgres.NewPaymentRepository(pool)
	pickupService := service.NewPickupService(pickupRepo, paymentRepo, txManager, clk, domain.Pricing{Standard: cfg.Pricing.Standard, Electronic: cfg.Pricing.Electronic})
	pickupHandler := handler.NewPickupHandler(pickupService)
	httpx.RegisterPickupRoutes(server.API(), pickupHandler, server.PickupRateLimit())

	return &App{
		cfg:       cfg,
		logger:    logger,
		pool:      pool,
		storage:   store,
		server:    server,
		scheduler: scheduler,
		limiter:   limiter,
		txManager: txManager,
		clock:     clk,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return a.server.Listen()
	})
	g.Go(func() error {
		return a.scheduler.Start(gctx)
	})
	g.Go(func() error {
		a.limiter.StartJanitor(gctx)
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()
		return a.server.Shutdown(shutdownCtx)
	})

	return g.Wait()
}

func (a *App) Close() {
	a.pool.Close()
}
