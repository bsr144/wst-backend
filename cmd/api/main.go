package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"wst-backend/config"
	"wst-backend/db"
	"wst-backend/internal/app"
)

func main() {
	var healthFlag bool
	var migrateFlag string
	flag.BoolVar(&healthFlag, "health", false, "probe the local /healthz endpoint and exit 0/1")
	flag.StringVar(&migrateFlag, "migrate", "", "run migrations (up|down) and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if healthFlag {
		os.Exit(healthProbe(cfg.AppPort))
	}

	logger := newLogger(cfg.LogLevel, cfg.AppEnv)
	defer func() { _ = logger.Sync() }()

	if migrateFlag != "" {
		if err := db.Migrate(cfg.Postgres.DSN(), migrateFlag); err != nil {
			logger.Fatal("migrate", zap.String("direction", migrateFlag), zap.Error(err))
		}
		logger.Info("migrate_done", zap.String("direction", migrateFlag))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("startup", zap.Error(err))
	}
	defer application.Close()

	logger.Info("starting", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv))
	if err := application.Run(ctx); err != nil {
		logger.Fatal("run", zap.Error(err))
	}
	logger.Info("stopped")
}

func healthProbe(port string) int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func newLogger(level, env string) *zap.Logger {
	var zc zap.Config
	if env == "local" || env == "development" {
		zc = zap.NewDevelopmentConfig()
	} else {
		zc = zap.NewProductionConfig()
	}
	if lvl, err := zap.ParseAtomicLevel(level); err == nil {
		zc.Level = lvl
	}
	logger, err := zc.Build()
	if err != nil {
		return zap.NewNop()
	}
	return logger
}
