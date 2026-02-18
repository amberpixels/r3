package r3mysql

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// MysqlCRUD is a CRUD repository based on database/sql with go-sql-driver/mysql.
// It embeds enginesql.BaseCRUD which provides the full r3.CRUD implementation.
// Create() is handled by BaseCRUD's LAST_INSERT_ID fallback (FlavorMySQL has
// SupportsRETURNING=false).
type MysqlCRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &MysqlCRUD[any, any]{}

// NewMysqlCRUD creates a new MySQL-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewMysqlCRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *MysqlCRUD[T, ID] {
	return &MysqlCRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorMySQL, opts...),
	}
}

// NewMysqlQuerier creates a read-only MySQL-based repository.
// Returns [r3.Querier] — a compile-time guarantee of read-only access.
func NewMysqlQuerier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewMysqlCRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *MysqlCRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *MysqlCRUD[T, ID]) DB() *sql.DB { return r.BaseCRUD.SQLDB() }
