package ports

import "errors"

// ErrNotFound is returned by repository methods when the requested entity does not exist.
// Callers should use errors.Is(err, ErrNotFound) to detect this condition.
var ErrNotFound = errors.New("entity not found")
