package r3gorm

import (
	"context"
	"errors"
	"sync"

	"github.com/amberpixels/r3"
	r3atoms "github.com/amberpixels/r3/atoms"
	"gorm.io/gorm"
)

// defaults stores the default values for repo params
type defaults struct {
	ListParams r3atoms.ListParams
	GetParams  r3atoms.GetParams
}

// GormCRUD is a richfull CRUD repository based on gorm.DB
type GormCRUD[T any, ID comparable] struct {
	db *gorm.DB

	defaults   defaults
	defaultsMu sync.RWMutex

	raw *GormRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GormCRUD[any, any]{}

// NewGormCRUD creates a new GORM-based
func NewGormCRUD[T any, ID comparable](db *gorm.DB) *GormCRUD[T, ID] {
	return &GormCRUD[T, ID]{
		db: db,
		defaults: defaults{
			ListParams: r3atoms.DefaultListParams(),
			GetParams:  r3atoms.DefaultGetParams(),
		},
		raw: NewGormRaw[T, ID](db),
	}
}

// SetDefaultParams sets default params for the
func (r *GormCRUD[T, ID]) SetDefaultParams(listParams r3atoms.ListParams) {
	r.defaultsMu.Lock()
	r.defaults.ListParams = listParams
	r.defaultsMu.Unlock()
}

// SetDefaultParams sets default params for the
func (r *GormCRUD[T, ID]) SetDefaultGetParams(getParams r3atoms.GetParams) {
	r.defaultsMu.Lock()
	r.defaults.GetParams = getParams
	r.defaultsMu.Unlock()
}

func (r *GormCRUD[T, ID]) getDefaultListParams() r3atoms.ListParams {
	r.defaultsMu.RLock()
	defer r.defaultsMu.Unlock()
	return r.defaults.ListParams
}

func (r *GormCRUD[T, ID]) getDefaultGetParams() r3atoms.GetParams {
	r.defaultsMu.RLock()
	defer r.defaultsMu.Unlock()
	return r.defaults.GetParams
}

func (r *GormCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) List(ctx context.Context, paramsArg ...r3atoms.ListParams) ([]T, int64, error) {
	var entities []T
	var totalCount int64

	// Merge params with defaults
	params := r.getDefaultListParams()
	for _, other := range paramsArg {
		params = params.MergeWith(other)
	}

	var entity T
	query := r.db.WithContext(ctx).Debug().Model(&entity)

	// Handle custom filters:
	if len(params.Filters) > 0 {
		for _, join := range params.Filters.ExtractJoins() {
			query = query.Joins(join)
		}

		safeSQL, args := params.Filters.DialectString()
		query = query.Where(safeSQL, args...)
	}

	// Sorting.
	for _, sort := range params.Sorts {
		query = query.Order(sort.DialectString())
	}

	// If Pagination is given, then we need to first Count all results without pagination:
	var isPaginated bool
	if params.Pagination.IsPaginated() {
		// We have to count first:
		if err := query.Count(&totalCount).Error; err != nil {
			return nil, 0, err
		}
		if totalCount == 0 {
			// no reason to call Find() if it has zero data
			return nil, 0, nil
		}

		// Then to add limit & offset for main Find() query
		limit, offset := params.Pagination.GetDialectLimitOffset()
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

func (r *GormCRUD[T, ID]) Get(ctx context.Context, id ID, paramsArg ...r3atoms.GetParams) (T, error) {
	var entity T

	// Merge params with defaults
	params := r.getDefaultGetParams()
	for _, other := range paramsArg {
		params = params.MergeWith(other)
	}

	// Apply preloads
	query := r.db.WithContext(ctx)
	for _, preload := range params.Preloads {
		query = query.Preload(preload.GetName(), preload.GetNestedPreloads())
	}

	// Fetch the record
	if params.IncludeTrashed.Some() {
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

func (r *GormCRUD[T, ID]) Patch(ctx context.Context, model T, fields r3atoms.Fields) (T, error) {
	var entity T

	// Validate the fields list (should not be empty)
	if len(fields) == 0 {
		return entity, errors.New("no fields specified for update")
	}

	// Perform partial update using GORM's `Select` to specify fields
	if err := r.db.WithContext(ctx).
		Model(&model).
		// Select(fields.Strings()). TODO(!)
		Updates(&model).Error; err != nil {
		return entity, err
	}

	// TODO: fetch again

	return entity, nil
}

func (r *GormCRUD[T, ID]) PatchRaw(ctx context.Context, id ID, patches ...r3atoms.Patch) (T, error) {
	var entity T

	// Validate the updates map (should not be empty)
	if len(patches) == 0 {
		return entity, errors.New("no fields to update")
	}

	// Build the raw updates map with field names as keys
	rawUpdates := map[string]any{}
	for field, value := range patches {
		_ = field
		_ = value
		// TODO FIXME
		//rawUpdates[field.String()] = value
	}

	// Perform the update using GORM's `Updates` method
	if err := r.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(rawUpdates).Error; err != nil {
		return entity, err
	}
	// TODO: re-fetch model
	return entity, nil
}

func (r *GormCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := r.db.WithContext(ctx).Delete(new(T), id).Error; err != nil {
		return err
	}
	return nil
}

func (r *GormCRUD[T, ID]) Raw() *GormRaw[T, ID] { return r.raw }
