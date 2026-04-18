package worker

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestRunner_AutoImportContinuesWhileProcessorIsBusy(t *testing.T) {
	t.Parallel()

	processorStarted := make(chan struct{}, 1)
	processorRelease := make(chan struct{})
	importCalled := make(chan struct{}, 1)

	runner := NewRunner(
		&stubRunnerProcessor{
			processNext: func(ctx context.Context) (bool, error) {
				select {
				case processorStarted <- struct{}{}:
				default:
				}
				select {
				case <-ctx.Done():
					return false, ctx.Err()
				case <-processorRelease:
					return false, nil
				}
			},
		},
		&stubAutoUploadImporter{
			importNext: func(ctx context.Context) (bool, error) {
				select {
				case importCalled <- struct{}{}:
				default:
				}
				<-ctx.Done()
				return false, ctx.Err()
			},
		},
		5*time.Millisecond,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runner.Run(ctx)
	}()

	select {
	case <-processorStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("processor did not start")
	}

	select {
	case <-importCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("auto-import did not run while processor was busy")
	}

	close(processorRelease)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop")
	}
}

type stubRunnerProcessor struct {
	mu                 sync.Mutex
	recoverInterrupted func(context.Context) error
	processNext        func(context.Context) (bool, error)
}

func (s *stubRunnerProcessor) RecoverInterruptedJobs(ctx context.Context) error {
	s.mu.Lock()
	fn := s.recoverInterrupted
	s.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

func (s *stubRunnerProcessor) ProcessNext(ctx context.Context) (bool, error) {
	s.mu.Lock()
	fn := s.processNext
	s.mu.Unlock()
	if fn == nil {
		return false, nil
	}
	return fn(ctx)
}

type stubAutoUploadImporter struct {
	importNext func(context.Context) (bool, error)
}

func (s *stubAutoUploadImporter) ImportNext(ctx context.Context) (bool, error) {
	if s.importNext == nil {
		return false, nil
	}
	return s.importNext(ctx)
}
