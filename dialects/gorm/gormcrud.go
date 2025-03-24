package r3gorm

import (
	"context"
	"errors"
	"sync"

	"github.com/amberpixels/r3"
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
	ListParams r3.ListParams
	GetParams  r3.GetParams
}

// NewGormCRUD creates a new GORM-based
func NewGormCRUD[T any, ID comparable](db *gorm.DB) *GormCRUD[T, ID] {
	return &GormCRUD[T, ID]{
		db: db,
		defaultParams: defaultParams{
			ListParams: r3.ListParams{}, // Add default values as needed
			GetParams:  r3.GetParams{},
		},
		raw: NewGormRaw[T, ID](db),
	}
}

// SetDefaultParams sets default params for the
func (r *GormCRUD[T, ID]) SetDefaultParams(listParams r3.ListParams) {
	r.defaultParamsM.Lock()
	defer r.defaultParamsM.Unlock()
	r.defaultParams.ListParams = listParams
}

// SetDefaultParams sets default params for the
func (r *GormCRUD[T, ID]) SetDefaultGetParams(getParams r3.GetParams) {
	r.defaultParamsM.Lock()
	defer r.defaultParamsM.Unlock()
	r.defaultParams.GetParams = getParams
}

// mergeListParams merges request-level params with repository-level default params.
func (r *GormCRUD[T, ID]) mergeListParams(paramsArg ...r3.ListParams) r3.ListParams {
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
	if params.Sort != nil {
		merged.Sort = params.Sort
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
func (r *GormCRUD[T, ID]) mergeGetParams(paramsArg ...r3.GetParams) r3.GetParams {
	r.defaultParamsM.RLock()
	defer r.defaultParamsM.RUnlock()

	// Merge logic: Fields, Preloads, etc.
	var params r3.GetParams
	if len(paramsArg) > 0 {
		params = paramsArg[0]
	}

	merged := params
	if params.Fields == nil || len(params.Fields.Fields()) == 0 {
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

func (r *GormCRUD[T, ID]) List(ctx context.Context, paramsArg ...r3.ListParams) ([]T, error) {
	var entities []T

	// Merge params with defaults
	params := r.mergeListParams(paramsArg...)

	// Apply filters, sorting, fields, and preloads
	query := r.db.WithContext(ctx)
	if params.Filters != nil {
		for _, filter := range params.Filters.GetFilters() {
			query = query.Where(filter.GetField().String()+" "+filter.GetOperator().String()+" ?", filter.GetValue())
		}
	}

	if params.Sort != nil {
		for _, sort := range params.Sort.GetSortCriterias() {
			query = query.Order(sort.String())
		}
	}

	if params.Fields != nil {
		if len(params.Fields.Fields()) > 0 {
			query = query.Select(params.Fields.Strings())
		}
	}

	for _, preload := range params.Preloads {
		query = query.Preload(preload.GetName(), preload.GetNestedPreloads())
	}
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	// Fetch the records
	if err := query.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (r *GormCRUD[T, ID]) Get(ctx context.Context, id ID, paramsArg ...r3.GetParams) (T, error) {
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

func (r *GormCRUD[T, ID]) Patch(ctx context.Context, model T, fields r3.Fieldables) (T, error) {
	var entity T

	// Validate the fields list (should not be empty)
	if fields == nil || len(fields.Fields()) == 0 {
		return entity, errors.New("no fields specified for update")
	}

	// Perform partial update using GORM's `Select` to specify fields
	if err := r.db.WithContext(ctx).
		Model(&model).
		Select(fields.Strings()).
		Updates(&model).Error; err != nil {
		return entity, err
	}

	// TODO: fetch again

	return entity, nil
}

func (r *GormCRUD[T, ID]) PatchRaw(ctx context.Context, id ID, patches ...r3.Patchable) (T, error) {
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
