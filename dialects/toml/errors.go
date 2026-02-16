package r3toml

import (
	"errors"
	"fmt"
)

// ErrTOMLDialectorFailed is a generic error for r3toml dialector failures.
var ErrTOMLDialectorFailed = errors.New("toml dialector failed")

// ErrTOMLNullValue is returned when trying to encode a nil value to TOML,
// which does not support null.
var ErrTOMLNullValue = errors.New("TOML does not support null values")

// newError creates a new error with the given error as a cause.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrTOMLDialectorFailed, err)
}

// IsErrDialectorFailure checks if the given error is a r3toml failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrTOMLDialectorFailed)
}
