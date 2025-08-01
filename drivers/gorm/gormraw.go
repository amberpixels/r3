package r3gorm

import (
	"context"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// GormRaw is a gorm wrapper that allows call any gorm request.
// Is considered to be embedded in GormCRUD.
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

// getTableName derives table name from generic type T.
func getTableName[T any]() string {
	var t T
	typ := reflect.TypeOf(t)

	// Handle pointer types
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Convert struct name to snake_case for table name
	return toSnakeCase(typ.Name())
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	const lowercaseBitMask = 32 // ASCII bit difference between uppercase and lowercase
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r | lowercaseBitMask) // Convert to lowercase
	}
	return result.String()
}

func (r *GormRaw[T, ID]) Find(ctx context.Context, cb func(*gorm.DB) *gorm.DB) ([]T, error) {
	var entities []T
	// Pass the context-aware db instance to the callback to build a custom query.
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
