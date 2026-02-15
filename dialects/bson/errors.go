package r3bson

import (
	"errors"
	"fmt"
)

// ErrBSONDialectorFailed is a generic error for r3bson dialector failures.
var ErrBSONDialectorFailed = errors.New("bson dialector failed")

// newError creates a new error with the given error as a cause.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrBSONDialectorFailed, err)
}

// IsErrDialectorFailure checks if the given error is a r3bson failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrBSONDialectorFailed)
}
