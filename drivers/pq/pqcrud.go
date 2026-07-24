package r3pq

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// PqCRUD is a lib/pq repository; enginesql.BaseCRUD supplies the r3.CRUD impl.
type PqCRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &PqCRUD[any, any]{}

// NewPqCRUD builds a repository. Map columns with `db:"column_name"` and mark the
// PK with `db:"id,pk"` (defaults to "id").
func NewPqCRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *PqCRUD[T, ID] {
	return &PqCRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorPostgres, opts...),
	}
}

// NewPqQuerier builds a read-only repository ([r3.Querier] enforces it).
func NewPqQuerier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewPqCRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *PqCRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *PqCRUD[T, ID]) DB() *sql.DB { return r.SQLDB() }
