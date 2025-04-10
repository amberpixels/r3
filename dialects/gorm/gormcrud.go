package r3gorm

import (
	"context"
	"errors"
	"sync"

	"github.com/amberpixels/r3"
	r3atoms "github.com/amberpixels/r3/atoms"
	"gorm.io/gorm"
)

// GormCRUD is a richfull CRUD repository based on gorm.DB
type GormCRUD[T any, ID comparable] struct {
	db *gorm.DB

	defaultParams  defaultParams
	defaultParamsM sync.RWMutex

	raw *GormRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GormCRUD[any, any]{}

type defaultParams struct {
	ListParams r3atoms.ListParams
	GetParams  r3atoms.GetParams
}

// NewGormCRUD creates a new GORM-based
func NewGormCRUD[T any, ID comparable](db *gorm.DB) *GormCRUD[T, ID] {
	return &GormCRUD[T, ID]{
		db: db,
		defaultParams: defaultParams{
			ListParams: r3atoms.ListParams{}, // Add default values as needed
			GetParams:  r3atoms.GetParams{},
		},
		raw: NewGormRaw[T, ID](db),
	}
}

// SetDefaultParams sets default params for the
func (r *GormCRUD[T, ID]) SetDefaultParams(listParams r3atoms.ListParams) {
	r.defaultParamsM.Lock()
	defer r.defaultParamsM.Unlock()
	r.defaultParams.ListParams = listParams
}

// SetDefaultParams sets default params for the
func (r *GormCRUD[T, ID]) SetDefaultGetParams(getParams r3atoms.GetParams) {
	r.defaultParamsM.Lock()
	defer r.defaultParamsM.Unlock()
	r.defaultParams.GetParams = getParams
}

// mergeListParams merges request-level params with repository-level default params.
func (r *GormCRUD[T, ID]) mergeListParams(paramsArg ...r3atoms.ListParams) r3atoms.ListParams {
	r.defaultParamsM.RLock()
	defer r.defaultParamsM.RUnlock()

	if len(paramsArg) == 0 {
		return r.defaultParams.ListParams.Clone()
	}
	params := paramsArg[0]

	merged := r.defaultParams.ListParams.Clone()

	if params.Filters != nil {
		merged.Filters = params.Filters
	}
	if params.Sorts != nil {
		merged.Sorts = params.Sorts
	}
	if params.Fields != nil {
		merged.Fields = params.Fields
	}

	if params.Preloads != nil {
		merged.Preloads = params.Preloads
	}
	merged.IncludeTrashed = merged.IncludeTrashed || r.defaultParams.ListParams.IncludeTrashed

	return merged
}

// mergeGetParams merges request-level params with repository-level default params.
func (r *GormCRUD[T, ID]) mergeGetParams(paramsArg ...r3atoms.GetParams) r3atoms.GetParams {
	r.defaultParamsM.RLock()
	defer r.defaultParamsM.RUnlock()

	// Merge logic: Fields, Preloads, etc.
	var params r3atoms.GetParams
	if len(paramsArg) > 0 {
		params = paramsArg[0]
	}

	merged := params
	if len(params.Fields) == 0 {
		merged.Fields = r.defaultParams.GetParams.Fields
	}
	if len(params.Preloads) == 0 {
		merged.Preloads = r.defaultParams.GetParams.Preloads
	}
	merged.IncludeTrashed = merged.IncludeTrashed || r.defaultParams.GetParams.IncludeTrashed

	return merged
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
	params := r.mergeListParams(paramsArg...)

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
		if err := query.Count(&totalCount).Error; err != nil {
			return nil, 0, err
		}

		if totalCount == 0 {
			// no reason to call Find() if it has zero data
			return nil, 0, nil
		}

		// Apply pagination
		query = query.Limit(params.Pagination.GetLimit(100)) // 100 for default
		query = query.Offset(params.Pagination.Offset)
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
	params := r.mergeGetParams(paramsArg...)

	// Apply preloads
	query := r.db.WithContext(ctx)
	for _, preload := range params.Preloads {
		query = query.Preload(preload.GetName(), preload.GetNestedPreloads())
	}

	// Fetch the record
	if params.IncludeTrashed {
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
