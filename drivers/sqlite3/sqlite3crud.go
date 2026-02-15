package r3sqlite3

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// Sqlite3CRUD is a CRUD repository based on database/sql with mattn/go-sqlite3.
// It embeds enginesql.BaseCRUD which provides the full r3.CRUD implementation.
type Sqlite3CRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &Sqlite3CRUD[any, any]{}

// NewSqlite3CRUD creates a new SQLite-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewSqlite3CRUD[T any, ID comparable](db *sql.DB) *Sqlite3CRUD[T, ID] {
	return &Sqlite3CRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorSQLite),
	}
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *Sqlite3CRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *Sqlite3CRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SQLDB() }
