package depogorm

import (
	"context"
	"errors"
	"sync"

	"github.com/amberpixels/depo"
	"gorm.io/gorm"
)

type GormRepository[T any, ID comparable] struct {
	db *gorm.DB

	defaultParams  RepositoryDefaultParams
	defaultParamsM sync.RWMutex
}

type RepositoryDefaultParams struct {
	ListParams depo.ListParams
	GetParams  depo.GetParams
}

// NewGormRepository creates a new GORM-based
func NewGormRepository[T any, ID comparable](db *gorm.DB) *GormRepository[T, ID] {
	return &GormRepository[T, ID]{
		db: db,
		defaultParams: RepositoryDefaultParams{
			ListParams: depo.ListParams{}, // Add default values as needed
			GetParams:  depo.GetParams{},
		},
	}
}

// SetDefaultParams sets default params for the
func (r *GormRepository[T, ID]) SetDefaultParams(params RepositoryDefaultParams) {
	r.defaultParamsM.Lock()
	defer r.defaultParamsM.Unlock()
	r.defaultParams = params
}

// mergeListParams merges request-level params with repository-level default params.
func (r *GormRepository[T, ID]) mergeListParams(params depo.ListParams) depo.ListParams {
	r.defaultParamsM.RLock()
	defer r.defaultParamsM.RUnlock()

	// Merge logic: Filters, Sort, Fields, etc.
	merged := params
	if params.Filters != nil {
		merged.Filters = r.defaultParams.ListParams.Filters
	}
	if params.Sort != nil {
		merged.Sort = r.defaultParams.ListParams.Sort
	}
	if params.Fields != nil {
		merged.Fields = r.defaultParams.ListParams.Fields
	}
	if len(params.Preloads) == 0 {
		merged.Preloads = r.defaultParams.ListParams.Preloads
	}
	merged.IncludeTrashed = merged.IncludeTrashed || r.defaultParams.ListParams.IncludeTrashed

	return merged
}

// mergeGetParams merges request-level params with repository-level default params.
func (r *GormRepository[T, ID]) mergeGetParams(params depo.GetParams) depo.GetParams {
	r.defaultParamsM.RLock()
	defer r.defaultParamsM.RUnlock()

	// Merge logic: Fields, Preloads, etc.
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

func (r *GormRepository[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormRepository[T, ID]) List(ctx context.Context, params depo.ListParams) ([]T, error) {
	var entities []T

	// Merge params with defaults
	params = r.mergeListParams(params)

	// Apply filters, sorting, fields, and preloads
	query := r.db.WithContext(ctx)
	if params.Filters != nil {
		for _, filter := range params.Filters.GetFilters() {
			query = query.Where(filter.GetField().String()+" "+filter.GetOperator().String(), filter.GetValue())
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

func (r *GormRepository[T, ID]) Get(ctx context.Context, id ID, params depo.GetParams) (T, error) {
	var entity T

	// Merge params with defaults
	params = r.mergeGetParams(params)

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

func (r *GormRepository[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	// Perform a full update using GORM's `Save` method, rewriting the entire model.
	if err := r.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormRepository[T, ID]) Patch(ctx context.Context, model T, fields depo.Fieldables) error {
	// Validate the fields list (should not be empty)
	if fields == nil || len(fields.Fields()) == 0 {
		return errors.New("no fields specified for update")
	}

	// Perform partial update using GORM's `Select` to specify fields
	if err := r.db.WithContext(ctx).
		Model(&model).
		Select(fields.Strings()).
		Updates(&model).Error; err != nil {
		return err
	}
	return nil
}

func (r *GormRepository[T, ID]) PatchRaw(ctx context.Context, id ID, updates map[depo.Fieldable]any) error {
	// Validate the updates map (should not be empty)
	if len(updates) == 0 {
		return errors.New("no fields to update")
	}

	// Build the raw updates map with field names as keys
	rawUpdates := map[string]any{}
	for field, value := range updates {
		rawUpdates[field.String()] = value
	}

	// Perform the update using GORM's `Updates` method
	if err := r.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(rawUpdates).Error; err != nil {
		return err
	}
	return nil
}

func (r *GormRepository[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := r.db.WithContext(ctx).Delete(new(T), id).Error; err != nil {
		return err
	}
	return nil
}
