package enginefile

import "github.com/amberpixels/r3"

// NewQuerier creates a read-only file-based repository.
// It returns [r3.Querier], providing a compile-time guarantee that the
// caller cannot perform write operations.
//
// Internally this constructs a full BaseCRUD but only exposes the
// Querier interface (Get, List).
func NewQuerier[T any, ID comparable](idGen IDGenerator[ID], opts ...Option) (r3.Querier[T, ID], error) {
	return New[T, ID](idGen, opts...)
}
