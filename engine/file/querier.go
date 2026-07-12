package enginefile

import "github.com/amberpixels/r3"

// NewQuerier creates a read-only file-based repository: it builds a full
// BaseCRUD but returns only [r3.Querier], so callers cannot write.
func NewQuerier[T any, ID comparable](idGen IDGenerator[ID], opts ...Option) (r3.Querier[T, ID], error) {
	return New[T, ID](idGen, opts...)
}
