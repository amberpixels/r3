package enginefile

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/amberpixels/r3"
)

// Compile-time interface check.
var _ r3.CRUD[any, any] = (*BaseCRUD[any, any])(nil)

// BaseCRUD implements r3.CRUD for file-based storage.
type BaseCRUD[T any, ID comparable] struct {
	r3.DefaultsManager

	Meta   StructMeta
	Config r3.Config
	store  storage
	codec  Codec
	idGen  IDGenerator[ID]
	mu     sync.RWMutex
}

// New creates a new file-based CRUD instance.
// An IDGenerator must be provided explicitly.
func New[T any, ID comparable](idGen IDGenerator[ID], opts ...Option) (*BaseCRUD[T, ID], error) {
	cfg := &config{
		baseDir: ".",
		codec:   JSONCodec(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	meta := GetStructMeta[T]()

	resolved := r3.ResolveOptions(cfg.r3Opts...)
	// The file engine does not apply value codecs yet; reject a declared codec
	// loudly rather than serializing the un-encoded value to the file.
	r3.RequireCodecSupport(r3.SchemaOf[T](r3.WithSchemaNaming(resolved.Config.Naming)), "r3/engine/file")

	var st storage
	switch {
	case cfg.filePath != "":
		// Explicit file path override.
		st = &singleFileStorage{filePath: cfg.filePath}
	case cfg.directoryMode:
		st = newDirectoryStorage(cfg.baseDir, meta.ResourceName, &meta)
	default:
		st = newSingleFileStorage(cfg.baseDir, meta.ResourceName, cfg.codec)
	}

	crud := &BaseCRUD[T, ID]{
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Config:          resolved.Config,
		store:           st,
		codec:           cfg.codec,
		idGen:           idGen,
	}

	return crud, nil
}

// loadAll loads all entities from storage.
func (r *BaseCRUD[T, ID]) loadAll() ([]T, error) {
	var entities []T
	if err := r.store.load(r.codec, &entities); err != nil {
		return nil, err
	}
	if entities == nil {
		entities = []T{}
	}
	return entities, nil
}

// saveAll writes all entities to storage.
func (r *BaseCRUD[T, ID]) saveAll(entities []T) error {
	return r.store.save(r.codec, entities)
}

// findByPK finds an entity index by primary key. Returns -1 if not found.
func (r *BaseCRUD[T, ID]) findByPK(entities []T, id ID) int {
	for i, e := range entities {
		if v := r.Meta.PKValue(e); v != nil {
			if pkID, ok := v.(ID); ok && pkID == id {
				return i
			}
		}
	}
	return -1
}

// isSoftDeleted checks if an entity is soft-deleted.
func (r *BaseCRUD[T, ID]) isSoftDeleted(entity T) bool {
	idx := r.Meta.SoftDeleteFieldIdx()
	if idx < 0 {
		return false
	}
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	field := v.Field(idx)
	return !field.IsZero()
}

// filterSoftDeleted removes soft-deleted entities from the slice.
func (r *BaseCRUD[T, ID]) filterSoftDeleted(entities []T, includeTrashed bool) []T {
	if r.Meta.SoftDeleteField == "" || includeTrashed {
		return entities
	}

	var result []T
	for _, e := range entities {
		if !r.isSoftDeleted(e) {
			result = append(result, e)
		}
	}
	return result
}

// Create inserts a new entity, assigning a generated ID.
func (r *BaseCRUD[T, ID]) Create(_ context.Context, entity T) (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entities, err := r.loadAll()
	if err != nil {
		var zero T
		return zero, fmt.Errorf("load entities: %w", err)
	}

	existingIDs := extractIDs[T, ID](entities, &r.Meta)
	newID, err := r.idGen.Generate(existingIDs)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("generate ID: %w", err)
	}

	r.Meta.SetPKValue(&entity, newID)
	entities = append(entities, entity)

	if err := r.saveAll(entities); err != nil {
		var zero T
		return zero, fmt.Errorf("save entities: %w", err)
	}

	return entity, nil
}

// Get retrieves an entity by its primary key.
func (r *BaseCRUD[T, ID]) Get(_ context.Context, id ID, qarg ...r3.Query) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := r.MergeGetQuery(qarg...)

	entities, err := r.loadAll()
	if err != nil {
		var zero T
		return zero, fmt.Errorf("load entities: %w", err)
	}

	idx := r.findByPK(entities, id)
	if idx < 0 {
		var zero T
		return zero, errNotFound
	}

	entity := entities[idx]

	// Check soft-delete
	includeTrashed := q.IncludeTrashed.Some() && q.IncludeTrashed.Unwrap()
	if r.Meta.SoftDeleteField != "" && !includeTrashed && r.isSoftDeleted(entity) {
		var zero T
		return zero, errNotFound
	}

	return entity, nil
}

// List retrieves entities matching the given query.
func (r *BaseCRUD[T, ID]) List(_ context.Context, qarg ...r3.Query) ([]T, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := r.MergeListQuery(qarg...)

	entities, err := r.loadAll()
	if err != nil {
		return nil, 0, fmt.Errorf("load entities: %w", err)
	}

	// Filter soft-deleted
	includeTrashed := q.IncludeTrashed.Some() && q.IncludeTrashed.Unwrap()
	entities = r.filterSoftDeleted(entities, includeTrashed)

	// Apply filters
	if len(q.Filters) > 0 {
		entities, err = filterEntities(entities, q.Filters, &r.Meta)
		if err != nil {
			return nil, 0, fmt.Errorf("apply filters: %w", err)
		}
	}

	// Count total after filtering (before pagination)
	totalCount := int64(len(entities))

	// Apply sorts
	sortEntities(entities, q.Sorts, &r.Meta)

	// Apply cursor pagination or offset pagination
	isCursorPaginated := q.Cursor != nil
	isPaginated := !isCursorPaginated && q.Pagination != nil && q.Pagination.IsPaginated()

	if isCursorPaginated {
		if len(q.Sorts) == 0 {
			return nil, 0, r3.ErrCursorRequiresSort
		}

		token := q.Cursor.Token()
		if token != "" {
			values, err := r3.DecodeCursor(token)
			if err != nil {
				return nil, 0, fmt.Errorf("decode cursor: %w", err)
			}
			entities = filterCursorPosition(entities, values, q.Sorts, q.Cursor.Direction(), &r.Meta)
		}

		limit := q.Cursor.GetLimit()
		if limit < len(entities) {
			entities = entities[:limit]
		}
	} else if isPaginated {
		limit, offset := q.Pagination.ToLimitOffset()
		if offset >= len(entities) {
			entities = nil
		} else {
			end := min(offset+limit, len(entities))
			entities = entities[offset:end]
		}
	}

	var result []T
	var count int64
	if isCursorPaginated {
		result, count = r3.FinalizeCountCursor(entities)
	} else {
		result, count = r3.FinalizeCount(entities, totalCount, isPaginated)
	}
	if result == nil {
		result = []T{}
	}
	return result, count, nil
}

// Count returns the number of entities matching the query's filters.
func (r *BaseCRUD[T, ID]) Count(_ context.Context, qarg ...r3.Query) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := r.MergeListQuery(qarg...)

	entities, err := r.loadAll()
	if err != nil {
		return 0, fmt.Errorf("load entities: %w", err)
	}

	includeTrashed := q.IncludeTrashed.Some() && q.IncludeTrashed.Unwrap()
	entities = r.filterSoftDeleted(entities, includeTrashed)

	if len(q.Filters) > 0 {
		entities, err = filterEntities(entities, q.Filters, &r.Meta)
		if err != nil {
			return 0, fmt.Errorf("apply filters: %w", err)
		}
	}

	return int64(len(entities)), nil
}

// Update replaces an existing entity entirely.
func (r *BaseCRUD[T, ID]) Update(_ context.Context, entity T) (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entities, err := r.loadAll()
	if err != nil {
		var zero T
		return zero, fmt.Errorf("load entities: %w", err)
	}

	pkVal := r.Meta.PKValue(entity)
	if pkVal == nil {
		var zero T
		return zero, errors.New("entity has no primary key value")
	}
	id, ok := pkVal.(ID)
	if !ok {
		var zero T
		return zero, errors.New("primary key type mismatch")
	}

	idx := r.findByPK(entities, id)
	if idx < 0 {
		var zero T
		return zero, errNotFound
	}

	entities[idx] = entity

	if err := r.saveAll(entities); err != nil {
		var zero T
		return zero, fmt.Errorf("save entities: %w", err)
	}

	return entity, nil
}

// Patch performs a partial update, modifying only the specified fields.
func (r *BaseCRUD[T, ID]) Patch(_ context.Context, entity T, fields r3.Fields) (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fieldNames := r3.FieldsToStrings(fields)
	if _, err := r.Meta.ValidatePatchFields(fieldNames); err != nil {
		var zero T
		return zero, err
	}

	entities, err := r.loadAll()
	if err != nil {
		var zero T
		return zero, fmt.Errorf("load entities: %w", err)
	}

	pkVal := r.Meta.PKValue(entity)
	if pkVal == nil {
		var zero T
		return zero, errors.New("entity has no primary key value")
	}
	id, ok := pkVal.(ID)
	if !ok {
		var zero T
		return zero, errors.New("primary key type mismatch")
	}

	idx := r.findByPK(entities, id)
	if idx < 0 {
		var zero T
		return zero, errNotFound
	}

	// Copy specified fields from input entity to existing entity
	r.Meta.CopyFieldValues(&entities[idx], &entity, fieldNames)

	if err := r.saveAll(entities); err != nil {
		var zero T
		return zero, fmt.Errorf("save entities: %w", err)
	}

	return entities[idx], nil
}

// Delete removes an entity by ID. Uses soft-delete if configured.
func (r *BaseCRUD[T, ID]) Delete(_ context.Context, id ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entities, err := r.loadAll()
	if err != nil {
		return fmt.Errorf("load entities: %w", err)
	}

	idx := r.findByPK(entities, id)
	if idx < 0 {
		return errNotFound
	}

	if r.Meta.SoftDeleteField != "" {
		// Soft delete: set the soft-delete field to current time
		sdIdx := r.Meta.SoftDeleteFieldIdx()
		if sdIdx >= 0 {
			v := reflect.ValueOf(&entities[idx]).Elem()
			field := v.Field(sdIdx)
			now := time.Now()
			// Support *time.Time and time.Time
			if field.Type() == reflect.TypeFor[*time.Time]() {
				field.Set(reflect.ValueOf(&now))
			} else if field.Type() == reflect.TypeFor[time.Time]() {
				field.Set(reflect.ValueOf(now))
			}
		}
	} else {
		// Hard delete: remove from slice
		entities = append(entities[:idx], entities[idx+1:]...)
	}

	if err := r.saveAll(entities); err != nil {
		return fmt.Errorf("save entities: %w", err)
	}

	return nil
}

// HardDelete permanently removes an entity regardless of soft-delete configuration.
func (r *BaseCRUD[T, ID]) HardDelete(_ context.Context, id ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entities, err := r.loadAll()
	if err != nil {
		return fmt.Errorf("load entities: %w", err)
	}

	idx := r.findByPK(entities, id)
	if idx < 0 {
		return errNotFound
	}

	entities = append(entities[:idx], entities[idx+1:]...)

	if err := r.saveAll(entities); err != nil {
		return fmt.Errorf("save entities: %w", err)
	}

	return nil
}

// Restore un-deletes a soft-deleted entity by clearing its soft-delete field.
func (r *BaseCRUD[T, ID]) Restore(_ context.Context, id ID) error {
	if r.Meta.SoftDeleteField == "" {
		return errors.New("soft-delete is not configured for this entity")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entities, err := r.loadAll()
	if err != nil {
		return fmt.Errorf("load entities: %w", err)
	}

	idx := r.findByPK(entities, id)
	if idx < 0 {
		return errNotFound
	}

	sdIdx := r.Meta.SoftDeleteFieldIdx()
	if sdIdx >= 0 {
		v := reflect.ValueOf(&entities[idx]).Elem()
		field := v.Field(sdIdx)
		field.Set(reflect.Zero(field.Type()))
	}

	if err := r.saveAll(entities); err != nil {
		return fmt.Errorf("save entities: %w", err)
	}

	return nil
}
