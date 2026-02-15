// Package r3pq provides an r3.CRUD[T, ID] driver backed by lib/pq,
// the pure Go PostgreSQL driver for database/sql.
//
// Driver: github.com/lib/pq
// Source: https://github.com/lib/pq
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (BaseRaw) for arbitrary SQL with $1-style placeholders
//
// Limitations / notes:
//   - No ORM layer: this driver builds raw SQL and uses reflection-based struct scanning.
//     Model structs must use `db` struct tags (e.g. `db:"column_name,pk"`).
//   - No preload support. Relations (joins, eager loading) must be done via Raw().
//   - No soft-delete support. IncludeTrashed is ignored.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - The r3 SQL dialect produces `?` placeholders; this driver converts them to
//     PostgreSQL-style `$1, $2, ...` numbered placeholders internally.
//   - Transactions are supported via the r3.Transactor interface (BeginTx).
//   - For advanced use cases (CTEs, etc.), use Raw().DB to access
//     the underlying *sql.DB directly.
package r3pq

import (
	"database/sql"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/sqlbase"
)

// PqCRUD is a CRUD repository based on database/sql with lib/pq.
// It embeds sqlbase.BaseCRUD which provides the full r3.CRUD implementation.
type PqCRUD[T any, ID comparable] struct {
	*sqlbase.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &PqCRUD[any, any]{}

// NewPqCRUD creates a new database/sql-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewPqCRUD[T any, ID comparable](db *sql.DB) *PqCRUD[T, ID] {
	return &PqCRUD[T, ID]{
		BaseCRUD: sqlbase.NewBaseCRUD[T, ID](db, sqlbase.FlavorPostgres),
	}
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *PqCRUD[T, ID]) Raw() *sqlbase.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *PqCRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SqlDB() }
