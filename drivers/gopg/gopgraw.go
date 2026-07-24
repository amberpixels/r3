package r3gopg

import (
	"context"
	"reflect"

	"github.com/go-pg/pg/v10/orm"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// GoPgRaw is the go-pg escape hatch, exposing any go-pg query via Find/Scan callbacks.
type GoPgRaw[T any, ID any] struct {
	db        orm.DB
	tableName string
}

// NewGoPgRaw creates a new GoPgRaw instance.
func NewGoPgRaw[T any, ID comparable](db orm.DB) *GoPgRaw[T, ID] {
	return &GoPgRaw[T, ID]{
		db:        db,
		tableName: getTableName[T](),
	}
}

// getTableName derives T's table name the go-pg way: snake_case and pluralized
// (e.g. City -> cities, Event -> events).
func getTableName[T any]() string {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return r3utils.ToSnakeCasePlural(typ.Name())
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
func (r *GoPgRaw[T, ID]) Scan(ctx context.Context, cb func(*orm.Query) *orm.Query, dest any) error {
	var entities []T
	query := r.db.ModelContext(ctx, &entities)
	query = cb(query)
	return query.Select(dest)
}
