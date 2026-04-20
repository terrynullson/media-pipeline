package ports_test

import (
	"errors"
	"fmt"
	"testing"

	"media-pipeline/internal/domain/ports"
)

func TestErrNotFound_ErrorsIs(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("wrap: %w", ports.ErrNotFound)
	if !errors.Is(wrapped, ports.ErrNotFound) {
		t.Fatalf("errors.Is(wrapped, ErrNotFound) = false, want true")
	}
}

func TestErrNotFound_OtherErrorNotMatched(t *testing.T) {
	t.Parallel()

	other := errors.New("some other error")
	if errors.Is(other, ports.ErrNotFound) {
		t.Fatalf("errors.Is(other, ErrNotFound) = true, want false")
	}
}
