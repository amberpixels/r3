package r3url

import (
	"errors"
	"fmt"
)

// ErrURLDialectorFailed is a generic error for r3url dialector failures.
var ErrURLDialectorFailed = errors.New("url dialector failed")

// newError creates a new error with the given error as a cause.
// It's considered to be used for all errors returned from inside r3url package.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrURLDialectorFailed, err)
}

// IsErrDialectorFailure checks if the given error is a r3url failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrURLDialectorFailed)
}
