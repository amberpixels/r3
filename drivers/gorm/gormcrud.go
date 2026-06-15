package r3gorm

import (
	"context"
	"errors"
	"reflect"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"gorm.io/gorm"
)

// GormCRUD is a CRUD repository based on gorm.DB.
type GormCRUD[T any, ID comparable] struct {
	db *gorm.DB

	enginesql.DefaultsManager

	Config r3.Config
	raw    *GormRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GormCRUD[any, any]{}

// NewGormCRUD creates a new GORM-based CRUD repository.
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewGormCRUD[T any, ID comparable](db *gorm.DB, opts ...r3.Option) *GormCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	return &GormCRUD[T, ID]{
		db:              db,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Config:          resolved.Config,
		raw:             NewGormRaw[T, ID](db),
	}
}

func (r *GormCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return entity, err
	}
	if err := r.syncAssociations(ctx, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := enginesql.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	var entity T
	query := r.db.WithContext(ctx).Model(&entity)

	// Apply fields selection
	if fieldCols := r3.FieldsToStrings(prep.Query.Fields); len(fieldCols) > 0 {
		query = query.Select(fieldCols)
	}

	// Split preloads: r3-managed relations are handled post-query, others use GORM's Preload
	r3Preloads, gormPreloads := splitPreloads[T](prep.Query.Preloads)
	for _, preload := range gormPreloads {
		query = query.Preload(preload.GetName())
	}

	// Handle soft-deleted records
	if prep.Query.IncludeTrashed.Some(true) {
		query = query.Unscoped()
	}

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

	// Run R3-managed preloads after main query
	if len(r3Preloads) > 0 && len(entities) > 0 {
		if err := runR3Preloads(r.db.WithContext(ctx), entities, r3Preloads); err != nil {
			return nil, 0, err
		}
	}

	entities, totalCount = r3.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

// Count returns the number of records matching the query's filters.
func (r *GormCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	prep, err := enginesql.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return 0, err
	}

	var entity T
	query := r.db.WithContext(ctx).Model(&entity)

	if prep.Query.IncludeTrashed.Some(true) {
		query = query.Unscoped()
	}
	for _, join := range prep.Joins() {
		query = query.Joins(join.String())
	}
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *GormCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	query := r.db.WithContext(ctx)

	// Apply fields selection
	if fieldCols := r3.FieldsToStrings(q.Fields); len(fieldCols) > 0 {
		query = query.Select(fieldCols)
	}

	// Split preloads: r3-managed relations are handled post-query, others use GORM's Preload
	r3Preloads, gormPreloads := splitPreloads[T](q.Preloads)
	for _, preload := range gormPreloads {
		query = query.Preload(preload.GetName())
	}

	// Handle soft-deleted records
	if q.IncludeTrashed.Some(true) {
		query = query.Unscoped()
	}

	if err := query.First(&entity, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity, r3.ErrNotFound
		}
		return entity, err
	}

	// Run R3-managed preloads after main query
	if len(r3Preloads) > 0 {
		entities := []T{entity}
		if err := runR3Preloads(r.db.WithContext(ctx), entities, r3Preloads); err != nil {
			return entity, err
		}
		entity = entities[0]
	}

	return entity, nil
}

func (r *GormCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	if err := r.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return entity, err
	}
	if err := r.syncAssociations(ctx, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	cols := r3.FieldsToStrings(fields)

	meta := enginesql.GetStructMeta[T]()
	cols, err := meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}

	if err := r.db.WithContext(ctx).Model(&entity).Select(cols).Updates(entity).Error; err != nil {
		return entity, err
	}

	// Re-fetch so the returned entity reflects DB-side changes (triggers,
	// updated_at, defaults), honoring the documented Patch contract.
	id, ok := meta.PKValue(entity).(ID)
	if !ok {
		return entity, nil
	}
	return r.Get(ctx, id)
}

func (r *GormCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := r.db.WithContext(ctx).Delete(new(T), id).Error; err != nil {
		return err
	}
	return nil
}

// Restore un-deletes a soft-deleted record by clearing its deleted_at field.
func (r *GormCRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	return r.db.WithContext(ctx).Unscoped().Model(new(T)).Where("id = ?", id).
		Update("deleted_at", nil).Error
}

// HardDelete permanently removes a record, bypassing GORM's soft-delete.
func (r *GormCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(new(T), id).Error
}

// syncAssociations syncs M2M and owned has-many relations after Create/Update.
// Uses direct SQL based on R3 relation metadata — no GORM association tags needed.
func (r *GormCRUD[T, ID]) syncAssociations(ctx context.Context, entityPtr *T) error {
	meta := enginesql.GetStructMeta[T]()
	if len(meta.Relations) == 0 {
		return nil
	}

	pkVal := meta.PKValue(entityPtr)
	entityVal := reflect.ValueOf(entityPtr).Elem()
	db := r.db.WithContext(ctx)

	for _, rel := range meta.Relations {
		// A nil relation slice means "not loaded": skip it so an Update that did
		// not load the relation never wipes existing rows. A non-nil slice
		// (including an empty one) is authoritative and is synced — an empty
		// slice intentionally clears the relation.
		relField := entityVal.Field(rel.FieldIndex)
		if relField.Kind() == reflect.Slice && relField.IsNil() {
			continue
		}

		switch {
		case rel.Kind == enginesql.RelManyToMany:
			if err := syncM2M(db, rel, pkVal, entityPtr); err != nil {
				return err
			}
		case rel.Kind == enginesql.RelHasMany && rel.Owned:
			if err := syncOwnedHasMany(db, rel, pkVal, entityPtr); err != nil {
				return err
			}
		}
	}

	return nil
}

// syncM2M replaces all join table rows for a many-to-many relation using direct SQL.
func syncM2M[T any](db *gorm.DB, rel enginesql.RelationMeta, pkVal any, entityPtr *T) error {
	// Delete existing join rows
	if err := db.Exec(
		"DELETE FROM "+rel.JoinTable+" WHERE "+rel.FKColumn+" = ?", pkVal,
	).Error; err != nil {
		return err
	}

	// Insert new join rows
	slice := reflect.ValueOf(entityPtr).Elem().Field(rel.FieldIndex)
	for i := range slice.Len() {
		childPK := rel.TargetMeta.PKValue(slice.Index(i).Interface())
		if err := db.Exec(
			"INSERT INTO "+rel.JoinTable+" ("+rel.FKColumn+", "+rel.RefColumn+") VALUES (?, ?)",
			pkVal, childPK,
		).Error; err != nil {
			return err
		}
	}

	return nil
}

// syncOwnedHasMany replaces all owned children for a has-many relation.
// Deletes all existing children by FK, then creates the new set.
func syncOwnedHasMany[T any](db *gorm.DB, rel enginesql.RelationMeta, pkVal any, entityPtr *T) error {
	// Delete all existing children with this FK
	childTable := rel.TargetMeta.TableName
	if err := db.Exec(
		"DELETE FROM "+childTable+" WHERE "+rel.FKColumn+" = ?", pkVal,
	).Error; err != nil {
		return err
	}

	// Insert new children with FK set
	slice := reflect.ValueOf(entityPtr).Elem().Field(rel.FieldIndex)
	if slice.Len() == 0 {
		return nil
	}

	// Find the FK field index on the child struct so we can set it
	fkFieldIdx := -1
	for i, col := range rel.TargetMeta.Columns {
		if col == rel.FKColumn {
			fkFieldIdx = rel.TargetMeta.Fields[i]
			break
		}
	}

	for i := range slice.Len() {
		child := slice.Index(i)
		// Zero out PK so GORM creates a new record
		pkFieldIdx := rel.TargetMeta.Fields[rel.TargetMeta.PKField]
		child.Field(pkFieldIdx).SetZero()
		// Set FK to parent's PK
		if fkFieldIdx >= 0 {
			child.Field(fkFieldIdx).Set(reflect.ValueOf(pkVal).Convert(child.Field(fkFieldIdx).Type()))
		}
	}

	return db.Create(slice.Interface()).Error
}

// NewGormQuerier creates a read-only GORM-based repository.
// Returns [r3.Querier] — a compile-time guarantee of read-only access.
func NewGormQuerier[T any, ID comparable](db *gorm.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewGormCRUD[T, ID](db, opts...)
}

// Raw returns the GormRaw escape hatch for custom queries.
func (r *GormCRUD[T, ID]) Raw() *GormRaw[T, ID] { return r.raw }

// DB returns the underlying *gorm.DB for advanced usage.
func (r *GormCRUD[T, ID]) DB() *gorm.DB { return r.db }
