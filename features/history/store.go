package history

import (
	"errors"
)

// Sentinel errors for history operations.
var (
	// ErrVersionNotFound is returned when a requested version does not exist.
	ErrVersionNotFound = errors.New("r3history: version not found")

	// ErrRecordNotFound is returned when a requested change record does not exist.
	ErrRecordNotFound = errors.New("r3history: record not found")

	// ErrNoHistory is returned when attempting to revert an entity with no history.
	ErrNoHistory = errors.New("r3history: no history for this record")
)
