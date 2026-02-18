package r3pgx

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// PgxCRUD is a CRUD repository based on database/sql with jackc/pgx (via pgx/v5/stdlib).
// It embeds enginesql.BaseCRUD which provides the full r3.CRUD implementation.
type PgxCRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &PgxCRUD[any, any]{}

// NewPgxCRUD creates a new pgx-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewPgxCRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *PgxCRUD[T, ID] {
	return &PgxCRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorPostgres, opts...),
	}
}

// NewPgxQuerier creates a read-only pgx-based repository.
// Returns [r3.Querier] — a compile-time guarantee of read-only access.
func NewPgxQuerier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewPgxCRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *PgxCRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *PgxCRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SQLDB() }
