package r3bson

import (
	"errors"
	"fmt"
)

// ErrBSONDialectorFailed is the sentinel wrapping every r3bson failure.
var ErrBSONDialectorFailed = errors.New("bson dialector failed")

// newError wraps err under ErrBSONDialectorFailed.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrBSONDialectorFailed, err)
}

// IsErrDialectorFailure reports whether err is a r3bson failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrBSONDialectorFailed)
}
