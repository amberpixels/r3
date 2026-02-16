package r3yaml

import (
	"errors"
	"fmt"
)

// ErrYAMLDialectorFailed is a generic error for r3yaml dialector failures.
var ErrYAMLDialectorFailed = errors.New("yaml dialector failed")

// newError creates a new error with the given error as a cause.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrYAMLDialectorFailed, err)
}

// IsErrDialectorFailure checks if the given error is a r3yaml failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrYAMLDialectorFailed)
}
