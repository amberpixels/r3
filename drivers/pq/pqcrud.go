package r3pq

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// PqCRUD is a CRUD repository based on database/sql with lib/pq.
// It embeds enginesql.BaseCRUD which provides the full r3.CRUD implementation.
type PqCRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &PqCRUD[any, any]{}

// NewPqCRUD creates a new database/sql-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewPqCRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *PqCRUD[T, ID] {
	return &PqCRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorPostgres, opts...),
	}
}

// NewPqQuerier creates a read-only pq-based repository.
// Returns [r3.Querier] — a compile-time guarantee of read-only access.
func NewPqQuerier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewPqCRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *PqCRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *PqCRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SQLDB() }
