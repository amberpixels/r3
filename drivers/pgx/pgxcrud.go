package r3pgx

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// PgxCRUD is a pgx repository (via pgx/v5/stdlib); enginesql.BaseCRUD supplies the
// r3.CRUD impl.
type PgxCRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &PgxCRUD[any, any]{}

// NewPgxCRUD builds a repository. Map columns with `db:"column_name"` and mark the
// PK with `db:"id,pk"` (defaults to "id").
func NewPgxCRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *PgxCRUD[T, ID] {
	return &PgxCRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorPostgres, opts...),
	}
}

// NewPgxQuerier builds a read-only repository ([r3.Querier] enforces it).
func NewPgxQuerier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewPgxCRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *PgxCRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *PgxCRUD[T, ID]) DB() *sql.DB { return r.SQLDB() }
