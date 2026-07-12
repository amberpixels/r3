package r3sql

import (
	"errors"
	"fmt"
)

// ErrSQLDialectorFailed is the sentinel wrapping every r3sql failure.
var ErrSQLDialectorFailed = errors.New("sql dialector failed")

// newError wraps err under ErrSQLDialectorFailed.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrSQLDialectorFailed, err)
}

// IsErrDialectorFailure reports whether err is a r3sql failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrSQLDialectorFailed)
}
