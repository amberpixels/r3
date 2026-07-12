package r3url

import (
	"errors"
	"fmt"
)

// ErrURLDialectorFailed wraps every error returned from r3url.
var ErrURLDialectorFailed = errors.New("url dialector failed")

// newError wraps err under [ErrURLDialectorFailed].
func newError(err error) error {
	return fmt.Errorf("%w: %w", ErrURLDialectorFailed, err)
}

// IsErrDialectorFailure reports whether err is an r3url failure.
func IsErrDialectorFailure(err error) bool {
	return errors.Is(err, ErrURLDialectorFailed)
}
