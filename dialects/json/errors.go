package r3json

import (
	"errors"
	"fmt"
)

// ErrJSONDialectorFailed is a generic error for r3json dialector failures.
var ErrJSONDialectorFailed = errors.New("json dialector failed")

// newError wraps err under ErrJSONDialectorFailed; used for every error the package returns.
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrJSONDialectorFailed, err)
}

// IsErrDialectorFailure checks if the given error is a r3json failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrJSONDialectorFailed)
}
