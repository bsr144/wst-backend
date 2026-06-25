package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"wst-backend/internal/core/port/in"
)

func AutoCancelOrganic(pickups in.PickupService, interval time.Duration, logger *zap.Logger) Job {
	return Job{
		Name:     "autocancel_organic",
		Interval: interval,
		Run: func(ctx context.Context) error {
			canceled, err := pickups.CancelStaleOrganic(ctx)
			if err != nil {
				return err
			}
			if canceled > 0 {
				logger.Info("autocancel_organic_swept", zap.Int("canceled", canceled))
			}
			return nil
		},
	}
}
