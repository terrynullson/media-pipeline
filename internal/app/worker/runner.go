package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type Runner struct {
	processor    *Processor
	pollInterval time.Duration
	logger       *slog.Logger
}

func NewRunner(processor *Processor, pollInterval time.Duration, logger *slog.Logger) *Runner {
	return &Runner{
		processor:    processor,
		pollInterval: pollInterval,
		logger:       logger,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	r.logger.Info("worker loop started", slog.Duration("poll_interval", r.pollInterval))
	if err := r.processor.RecoverInterruptedJobs(ctx); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				r.logger.Info("worker loop stopped")
				return nil
			}
			return err
		}

		processed, err := r.processor.ProcessNext(ctx)
		if err != nil {
			r.logger.Error("worker iteration failed", slog.Any("error", err))
		}
		if processed {
			continue
		}

		r.logger.Debug("no pending jobs found")
		select {
		case <-ctx.Done():
			r.logger.Info("worker loop stopped")
			return nil
		case <-time.After(r.pollInterval):
		}
	}
}
