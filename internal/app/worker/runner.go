package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type AutoUploadImporter interface {
	ImportNext(ctx context.Context) (bool, error)
}

type Runner struct {
	processor    *Processor
	autoImporter AutoUploadImporter
	pollInterval time.Duration
	logger       *slog.Logger
}

func NewRunner(processor *Processor, autoImporter AutoUploadImporter, pollInterval time.Duration, logger *slog.Logger) *Runner {
	return &Runner{
		processor:    processor,
		autoImporter: autoImporter,
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

		if r.autoImporter != nil {
			imported, err := r.autoImporter.ImportNext(ctx)
			if err != nil {
				r.logger.Error("auto-upload iteration failed", slog.Any("error", err))
			}
			if imported {
				continue
			}
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
