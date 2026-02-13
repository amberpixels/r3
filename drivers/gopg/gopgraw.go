package r3gopg

import (
	"context"
	"reflect"

	"github.com/amberpixels/r3/sqlbase"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

// GoPgRaw is a go-pg wrapper that allows calling any go-pg query.
// Is considered to be embedded in GoPgCRUD.
type GoPgRaw[T any, ID any] struct {
	db        *pg.DB
	tableName string
}

// NewGoPgRaw creates a new GoPgRaw instance.
func NewGoPgRaw[T any, ID comparable](db *pg.DB) *GoPgRaw[T, ID] {
	return &GoPgRaw[T, ID]{
		db:        db,
		tableName: getTableName[T](),
	}
}

// getTableName derives table name from generic type T using go-pg conventions.
// go-pg pluralizes table names by default (e.g., City -> cities, Event -> events).
func getTableName[T any]() string {
	var t T
	typ := reflect.TypeOf(t)

	// Handle pointer types
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	// Convert struct name to snake_case and pluralize for table name
	return sqlbase.ToSnakeCasePlural(typ.Name())
}

// Find executes a custom query and returns the results.
// The callback receives a *orm.Query to build a custom query.
func (r *GoPgRaw[T, ID]) Find(ctx context.Context, cb func(*orm.Query) *orm.Query) ([]T, error) {
	var entities []T
	query := r.db.ModelContext(ctx, &entities)
	query = cb(query)
	if err := query.Select(); err != nil {
		return nil, err
	}
	return entities, nil
}

// Scan executes a custom query and scans results into the provided destination.
func (r *GoPgRaw[T, ID]) Scan(ctx context.Context, cb func(*orm.Query) *orm.Query, dest interface{}) error {
	var entities []T
	query := r.db.ModelContext(ctx, &entities)
	query = cb(query)
	return query.Select(dest)
}
