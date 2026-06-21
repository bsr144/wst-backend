package worker

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

type Scheduler struct {
	jobs   []Job
	logger *zap.Logger
}

func New(logger *zap.Logger, jobs ...Job) *Scheduler {
	return &Scheduler{jobs: jobs, logger: logger}
}

func (s *Scheduler) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, job := range s.jobs {
		wg.Add(1)
		go s.runJob(ctx, job, &wg)
	}
	wg.Wait()
	return nil
}

func (s *Scheduler) runJob(ctx context.Context, job Job, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, job.Interval)
			if err := job.Run(runCtx); err != nil {
				s.logger.Error("worker_job_failed", zap.String("job", job.Name), zap.Error(err))
			}
			cancel()
		}
	}
}
