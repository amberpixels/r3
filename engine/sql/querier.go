package enginesql

import (
	"database/sql"

	"github.com/amberpixels/r3"
)

// NewBaseQuerier builds a full BaseCRUD but returns only [r3.Querier] (Get,
// List), giving a compile-time guarantee that the caller cannot write.
func NewBaseQuerier[T any, ID comparable](db *sql.DB, flavor Flavor, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBaseCRUD[T, ID](db, flavor, opts...)
}
