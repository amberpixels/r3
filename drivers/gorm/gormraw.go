package r3gorm

import (
	"context"
	"reflect"

	r3utils "github.com/amberpixels/r3/internal/utils"
	"gorm.io/gorm"
)

// GormRaw is the GORM escape hatch, exposing any gorm query via Find/Scan callbacks.
type GormRaw[T any, ID any] struct {
	db        *gorm.DB
	tableName string
}

func NewGormRaw[T any, ID comparable](db *gorm.DB) *GormRaw[T, ID] {
	return &GormRaw[T, ID]{
		db:        db,
		tableName: getTableName[T](),
	}
}

// getTableName derives T's table name the GORM way: singular snake_case
// (e.g. City -> city).
func getTableName[T any]() string {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return r3utils.ToSnakeCase(typ.Name())
}

func (r *GormRaw[T, ID]) Find(ctx context.Context, cb func(*gorm.DB) *gorm.DB) ([]T, error) {
	var entities []T
	query := cb(r.db.WithContext(ctx).Table(r.tableName))
	if err := query.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (r *GormRaw[T, ID]) Scan(ctx context.Context, cb func(*gorm.DB) *gorm.DB, dest any) error {
	query := cb(r.db.WithContext(ctx).Table(r.tableName))
	return query.Scan(dest).Error
}
