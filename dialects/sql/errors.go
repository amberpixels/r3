package r3sql

import (
	"errors"
	"fmt"
)

// ErrSQLDialectorFailed is a generic error for r3sql dialector failures.
var ErrSQLDialectorFailed = errors.New("sql dialector failed")

// newError creates a new error with the given error as a cause.
// It's considered to be used for all errors returned from inside r3sql package.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrSQLDialectorFailed, err)
}

// IsErrDialectorFailure checks if the given error is a r3sql failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrSQLDialectorFailed)
}
