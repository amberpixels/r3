package r3gorm

import (
	"context"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/sqlbase"
	"gorm.io/gorm"
)

// GormCRUD is a CRUD repository based on gorm.DB.
type GormCRUD[T any, ID comparable] struct {
	db *gorm.DB

	sqlbase.DefaultsManager

	raw *GormRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GormCRUD[any, any]{}

// NewGormCRUD creates a new GORM-based CRUD repository.
func NewGormCRUD[T any, ID comparable](db *gorm.DB) *GormCRUD[T, ID] {
	return &GormCRUD[T, ID]{
		db:              db,
		DefaultsManager: sqlbase.NewDefaultsManager(),
		raw:             NewGormRaw[T, ID](db),
	}
}

func (r *GormCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := sqlbase.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	var entity T
	query := r.db.WithContext(ctx).Model(&entity)

	// Apply joins
	for _, join := range prep.Joins() {
		query = query.Joins(join.String())
	}

	// Apply filters
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	// Apply sorts
	for _, sort := range prep.Sorts {
		query = query.Order(sort.String())
	}

	// Pagination: count first, then limit/offset
	var totalCount int64
	if prep.IsPaginated {
		if err := query.Count(&totalCount).Error; err != nil {
			return nil, 0, err
		}
		if totalCount == 0 {
			return nil, 0, nil
		}
		query = query.Limit(prep.Limit).Offset(prep.Offset)
	}

	var entities []T
	if err := query.Find(&entities).Error; err != nil {
		return nil, 0, err
	}

	entities, totalCount = sqlbase.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

func (r *GormCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	query := r.db.WithContext(ctx)

	// Apply preloads
	for _, preload := range q.Preloads {
		query = query.Preload(preload.GetName())
	}

	// Handle soft-deleted records
	if q.IncludeTrashed.Some() {
		// TODO: check bool value (it can be negative!)
		query = query.Unscoped()
	}

	if err := query.First(&entity, id).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := r.db.WithContext(ctx).Delete(new(T), id).Error; err != nil {
		return err
	}
	return nil
}

// Raw returns the GormRaw escape hatch for custom queries.
func (r *GormCRUD[T, ID]) Raw() *GormRaw[T, ID] { return r.raw }

// DB returns the underlying *gorm.DB for advanced usage.
func (r *GormCRUD[T, ID]) DB() *gorm.DB { return r.db }
