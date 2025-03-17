package depogorm

import (
	"context"

	"gorm.io/gorm"
)

// GormRaw is a gorm wrapper that allows call any gorm request
// Is considered to be embedded in GormCRUD
type GormRaw[T any, ID any] struct {
	db        *gorm.DB
	tableName string
}

func NewGormRaw[T any, ID comparable](db *gorm.DB) *GormRaw[T, ID] {
	return &GormRaw[T, ID]{
		db:        db,
		tableName: "locations", // todo
	}
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
