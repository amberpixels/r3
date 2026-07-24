package r3mysql

import (
	"database/sql"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// MysqlCRUD is a go-sql-driver/mysql repository; enginesql.BaseCRUD supplies the
// r3.CRUD impl. Create goes through BaseCRUD's LAST_INSERT_ID fallback, since
// FlavorMySQL reports SupportsRETURNING=false.
type MysqlCRUD[T any, ID comparable] struct {
	*enginesql.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &MysqlCRUD[any, any]{}

// NewMysqlCRUD builds a repository. Map columns with `db:"column_name"` and mark
// the PK with `db:"id,pk"` (defaults to "id").
func NewMysqlCRUD[T any, ID comparable](db *sql.DB, opts ...r3.Option) *MysqlCRUD[T, ID] {
	return &MysqlCRUD[T, ID]{
		BaseCRUD: enginesql.NewBaseCRUD[T, ID](db, enginesql.FlavorMySQL, opts...),
	}
}

// NewMysqlQuerier builds a read-only repository ([r3.Querier] enforces it).
func NewMysqlQuerier[T any, ID comparable](db *sql.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewMysqlCRUD[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom queries.
func (r *MysqlCRUD[T, ID]) Raw() *enginesql.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *MysqlCRUD[T, ID]) DB() *sql.DB { return r.SQLDB() }
