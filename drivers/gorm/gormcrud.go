package r3gorm

import (
	"context"
	"fmt"
	"sync"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"gorm.io/gorm"
)

// defaults stores the default values for repo q.
type defaults struct {
	ListQuery r3.Query
	GetQuery  r3.Query
}

// GormCRUD is a richfull CRUD repository based on gorm.DB.
type GormCRUD[T any, ID comparable] struct {
	db *gorm.DB

	defaults   defaults
	defaultsMu sync.RWMutex

	raw *GormRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GormCRUD[any, any]{}

// NewGormCRUD creates a new GORM-based.
func NewGormCRUD[T any, ID comparable](db *gorm.DB) *GormCRUD[T, ID] {
	return &GormCRUD[T, ID]{
		db: db,
		defaults: defaults{
			ListQuery: r3.DefaultQuery(),
			GetQuery:  r3.DefaultQuery(),
		},
		raw: NewGormRaw[T, ID](db),
	}
}

// SetDefaultListQuery sets default ListQuery.
func (r *GormCRUD[T, ID]) SetDefaultListQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.ListQuery = q
	r.defaultsMu.Unlock()
}

// SetDefaultGetQuery sets default GetQuery.
func (r *GormCRUD[T, ID]) SetDefaultGetQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.GetQuery = q
	r.defaultsMu.Unlock()
}

func (r *GormCRUD[T, ID]) getDefaultListQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.ListQuery
}

func (r *GormCRUD[T, ID]) getDefaultGetQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.GetQuery
}

func (r *GormCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	var entities []T
	var totalCount int64

	// Merge query with defaults
	q := r.getDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	var entity T
	query := r.db.WithContext(ctx).Debug().Model(&entity)

	// TODO:
	// FIELDS

	// Handle custom filters:
	clauses, err := r3sql.FiltersToSQLClauses(q.Filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert filters to SQL: %w", err)
	}

	if len(clauses) > 0 {
		for _, join := range clauses.Joins() {
			query = query.Joins(join.String())
		}

		for _, clause := range clauses {
			query = query.Where(clause.Clause, clause.Args...)
		}
	}

	// Sorting.
	// sorts, err := r3sql.NewSorts(q.Sorts)
	// if err != nil {
	// 	return nil, 0, fmt.Errorf("failed to convert sorts to SQL: %w", err)
	// }

	// for _, sort := range sorts {
	// 	query = query.Order(sort.ToSQL()) // TODO: to Gorm objects instead of plain SQL
	// }

	// If Pagination is given, then we need to first Count all results without pagination:
	var isPaginated bool
	if paginationSpec, ok := q.Pagination.(*r3.PaginationSpec); ok && paginationSpec.IsPaginated() {
		// We have to count first:
		if err := query.Count(&totalCount).Error; err != nil {
			return nil, 0, err
		}
		if totalCount == 0 {
			// no reason to call Find() if it has zero data
			return nil, 0, nil
		}

		// Then to add limit & offset for main Find() query
		limit, offset := paginationSpec.ToLimitOffset()
		query = query.Limit(limit)
		query = query.Offset(offset)
		isPaginated = true
	}

	if err := query.Find(&entities).Error; err != nil {
		return nil, 0, err
	}

	// If there was no pagination, let's simply count results
	if !isPaginated {
		totalCount = int64(len(entities))
	}

	return entities, totalCount, nil
}

func (r *GormCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	// Merge query with defaults
	q := r.getDefaultGetQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	// Apply preloads
	query := r.db.WithContext(ctx)
	for _, preload := range q.Preloads {
		// For now, only use simple preloads - nested preloads are not implemented yet
		query = query.Preload(preload.GetName())
		// TODO(future): Handle nested preloads when GetNestedPreloads() is implemented
	}

	// Fetch the record
	if q.IncludeTrashed.Some() {
		// TODO: it can be negative!
		query = query.Unscoped() // Include soft-deleted records
	}
	if err := query.First(&entity, id).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	// Perform a full update using GORM's `Save` method, rewriting the entire model.
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

func (r *GormCRUD[T, ID]) Raw() *GormRaw[T, ID] { return r.raw }
