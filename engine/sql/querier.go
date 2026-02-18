package enginesql

import (
	"database/sql"

	"github.com/amberpixels/r3"
)

// NewBaseQuerier creates a read-only repository backed by database/sql.
// It returns [r3.Querier], providing a compile-time guarantee that the
// caller cannot perform write operations.
//
// Internally this constructs a full BaseCRUD but only exposes the
// Querier interface (Get, List).
func NewBaseQuerier[T any, ID comparable](db *sql.DB, flavor Flavor, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBaseCRUD[T, ID](db, flavor, opts...)
}
