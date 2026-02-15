// Package r3sqlite3 provides an r3.CRUD[T, ID] driver backed by mattn/go-sqlite3,
// the CGo SQLite3 driver for database/sql.
//
// Driver: github.com/mattn/go-sqlite3
// Source: https://github.com/mattn/go-sqlite3
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (BaseRaw) for arbitrary SQL with ?-style placeholders
//
// Limitations / notes:
//   - No ORM layer: this driver builds raw SQL and uses reflection-based struct scanning.
//     Model structs must use `db` struct tags (e.g. `db:"column_name,pk"`).
//   - No preload support. Relations (joins, eager loading) must be done via Raw().
//   - No soft-delete support. IncludeTrashed is ignored.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - Uses `?` placeholders natively (no conversion needed).
//   - ILIKE is not supported by SQLite; use LIKE instead (SQLite LIKE is case-insensitive
//     for ASCII characters by default).
//   - RETURNING clause requires SQLite 3.35+ (supported by go-sqlite3).
//   - Transactions are supported via the r3.Transactor interface (BeginTx).
//   - For advanced use cases (CTEs, etc.), use Raw().DB to access
//     the underlying *sql.DB directly.
package r3sqlite3

import (
	"database/sql"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/sqlbase"
)

// Sqlite3CRUD is a CRUD repository based on database/sql with mattn/go-sqlite3.
// It embeds sqlbase.BaseCRUD which provides the full r3.CRUD implementation.
type Sqlite3CRUD[T any, ID comparable] struct {
	*sqlbase.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &Sqlite3CRUD[any, any]{}

// NewSqlite3CRUD creates a new SQLite-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewSqlite3CRUD[T any, ID comparable](db *sql.DB) *Sqlite3CRUD[T, ID] {
	return &Sqlite3CRUD[T, ID]{
		BaseCRUD: sqlbase.NewBaseCRUD[T, ID](db, sqlbase.FlavorSQLite),
	}
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *Sqlite3CRUD[T, ID]) Raw() *sqlbase.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *Sqlite3CRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SqlDB() }
