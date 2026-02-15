// Package r3mysql provides an r3.CRUD[T, ID] driver backed by go-sql-driver/mysql,
// the pure Go MySQL driver for database/sql.
//
// Driver: github.com/go-sql-driver/mysql
// Source: https://github.com/go-sql-driver/mysql
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
//   - MySQL does not support RETURNING; Create uses INSERT + LAST_INSERT_ID() + SELECT
//     to return the full created entity including auto-generated columns.
//   - ILIKE is not supported by MySQL; use LIKE instead (case sensitivity depends on
//     column collation; default utf8mb4_0900_ai_ci is case-insensitive).
//   - NULLS FIRST/LAST is not natively supported by MySQL.
//   - DSN must include `parseTime=true` for time.Time scanning to work correctly.
//   - Transactions are supported via the r3.Transactor interface (BeginTx).
//   - For advanced use cases (CTEs, etc.), use Raw().DB to access
//     the underlying *sql.DB directly.
package r3mysql

import (
	"database/sql"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/sqlbase"
)

// MysqlCRUD is a CRUD repository based on database/sql with go-sql-driver/mysql.
// It embeds sqlbase.BaseCRUD which provides the full r3.CRUD implementation.
// Create() is handled by BaseCRUD's LAST_INSERT_ID fallback (FlavorMySQL has
// SupportsRETURNING=false).
type MysqlCRUD[T any, ID comparable] struct {
	*sqlbase.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &MysqlCRUD[any, any]{}

// NewMysqlCRUD creates a new MySQL-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewMysqlCRUD[T any, ID comparable](db *sql.DB) *MysqlCRUD[T, ID] {
	return &MysqlCRUD[T, ID]{
		BaseCRUD: sqlbase.NewBaseCRUD[T, ID](db, sqlbase.FlavorMySQL),
	}
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *MysqlCRUD[T, ID]) Raw() *sqlbase.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *MysqlCRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SqlDB() }
