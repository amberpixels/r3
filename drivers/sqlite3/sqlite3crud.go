package r3sqlite3

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// Sqlite3CRUD is a mattn/go-sqlite3 repository; enginesql.BaseCRUD supplies the
// r3.CRUD impl.
type Sqlite3CRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &Sqlite3CRUD[any, any]{}

// NewSqlite3CRUD builds a repository. Map columns with `db:"column_name"` and mark
// the PK with `db:"id,pk"` (defaults to "id").
func NewSqlite3CRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *Sqlite3CRUD[T, ID] {
	return &Sqlite3CRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorSQLite, opts...),
	}
}

// NewSqlite3Querier builds a read-only repository ([r3.Querier] enforces it).
func NewSqlite3Querier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewSqlite3CRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *Sqlite3CRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *Sqlite3CRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SQLDB() }
