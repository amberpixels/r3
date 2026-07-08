package r3gorm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormCRUD is a CRUD repository based on gorm.DB.
type GormCRUD[T any, ID comparable] struct {
	db *gorm.DB

	enginesql.DefaultsManager

	meta   enginesql.StructMeta
	schema r3.Schema // logical schema for read validation and write-shaping
	Config r3.Config
	raw    *GormRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GormCRUD[any, any]{}
var _ r3.Aggregator = &GormCRUD[any, any]{}

// NewGormCRUD creates a new GORM-based CRUD repository.
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewGormCRUD[T any, ID comparable](db *gorm.DB, opts ...r3.Option) *GormCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	return &GormCRUD[T, ID]{
		db:              db,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		meta:            enginesql.GetStructMeta[T](),
		schema:          r3.SchemaOf[T](r3.WithSchemaNaming(resolved.Config.Naming)),
		Config:          resolved.Config,
		raw:             NewGormRaw[T, ID](db),
	}
}

func (r *GormCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	db := r.db.WithContext(ctx)
	// Omit non-creatable columns (readonly feed columns, the soft-delete column)
	// so the DB fills their defaults. The PK is never omitted, so GORM keeps
	// handling it (auto-increment or a caller-set key) exactly as before. Managed
	// timestamps are stamped with server time and written, not omitted.
	if omit := r.writeOmit(ctx, &entity, r3.WriteOpCreate); len(omit) > 0 {
		db = db.Omit(omit...)
	}
	if err := db.Create(&entity).Error; err != nil {
		return entity, err
	}
	if err := r.syncAssociations(ctx, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

// writeOmit returns the non-PK columns to Omit from a GORM write for the given
// op, and stamps the entity's managed timestamp columns (created_at/updated_at)
// with server time. Managed timestamps are kept OUT of the omit set so GORM
// writes the engine-set values — they are system-written, not omitted. The omit
// set is empty (and no stamping happens) when the write guard is bypassed or no
// schema is present, so the write proceeds unrestricted with caller values.
func (r *GormCRUD[T, ID]) writeOmit(ctx context.Context, entityPtr *T, op r3.WriteOp) []string {
	nonPK := r.meta.NonPKColumns()
	writable := enginesql.WriteColumns(ctx, r.schema, nonPK, op)
	if len(writable) == len(nonPK) {
		return nil // unrestricted (bypass or zero schema): caller controls everything
	}

	allowed := make(map[string]bool, len(writable))
	for _, c := range writable {
		allowed[c] = true
	}
	// Managed timestamps are system-written: stamp them and treat as writable.
	managed := enginesql.ManagedTimestampColumns(r.meta, r.Config.Naming, op)
	enginesql.SetTimeColumns(r.meta, entityPtr, managed, time.Now())
	for _, c := range managed {
		allowed[c] = true
	}

	var omit []string
	for _, c := range nonPK {
		if !allowed[c] {
			omit = append(omit, c)
		}
	}
	return omit
}

func (r *GormCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := r.prepareList(ctx, qarg...)
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
	prep, err := r.prepareList(ctx, qarg...)
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

// Aggregate computes grouped aggregates over the records matching the query.
// See r3.Aggregator for the query semantics.
func (r *GormCRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	merged := r.MergeListQuery(qarg...)
	if hasRelationFilters(merged.Filters) {
		lowered, err := lowerRelationFilters[T](ctx, r.db, merged.Filters)
		if err != nil {
			return nil, err
		}
		merged.Filters = lowered
	}
	prep, err := enginesql.PrepareMergedAggregateQuery(r.schema, merged)
	if err != nil {
		return nil, err
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

	query = query.Select(strings.Join(prep.SelectList, ", "))
	// Group by raw field names one at a time: gorm quotes each per its dialect
	// (a pre-quoted name would be quoted twice; a joined "a, b" string only
	// works by accident of gorm's raw-string comma heuristic).
	for _, g := range prep.Query.GroupBy {
		query = query.Group(g.String())
	}
	if prep.Having.Clause != "" {
		query = query.Having(prep.Having.Clause, prep.Having.Args...)
	}
	for _, sort := range prep.Sorts {
		query = query.Order(sort.String())
	}
	if prep.IsPaginated {
		query = query.Limit(prep.Limit).Offset(prep.Offset)
	}

	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return enginesql.ScanAggregateRows(rows)
}

func (r *GormCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	meta := r.meta
	q := r.MergeGetQuery(qarg...)
	if err := r.schema.ValidateQuery(q); err != nil {
		return entity, err
	}

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

	// Match the PK explicitly (clause.Eq binds id as a parameter). Passing id as
	// a gorm cond — First(&entity, id) — is unsafe for non-integer keys: gorm
	// treats a string id as a raw SQL expression (e.g. "rec-1" -> rec - 1).
	if err := query.Where(clause.Eq{Column: meta.PKColumn, Value: id}).
		First(&entity).Error; err != nil {
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
	db := r.db.WithContext(ctx)
	// Omit non-mutable columns (created_at, readonly feed columns, the
	// soft-delete column) so a full Update can't clobber them or resurrect a
	// soft-deleted row, while updated_at is stamped with server time and written.
	// This keeps Save's semantics (associations, zero-value writes) for every
	// other column. With nothing to protect (or under a bypass), Omit is empty
	// and this is the original full Save.
	if omit := r.writeOmit(ctx, &entity, r3.WriteOpMutate); len(omit) > 0 {
		db = db.Omit(omit...)
	}
	if err := db.Save(&entity).Error; err != nil {
		return entity, err
	}
	if err := r.syncAssociations(ctx, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GormCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	cols := r3.FieldsToStrings(fields)

	meta := r.meta
	cols, err := meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}
	if err := enginesql.RequireMutableColumns(ctx, r.schema, cols); err != nil {
		return entity, err
	}
	// A partial update still bumps updated_at (server time), like a full Update.
	if !r.schema.IsZero() && !r3.WriteGuardBypassed(ctx) {
		if managed := enginesql.ManagedTimestampColumns(meta, r.Config.Naming, r3.WriteOpMutate); len(managed) > 0 {
			enginesql.SetTimeColumns(meta, &entity, managed, time.Now())
			cols = append(cols, managed...)
		}
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
	meta := r.meta
	if err := r.db.WithContext(ctx).
		Where(clause.Eq{Column: meta.PKColumn, Value: id}).
		Delete(new(T)).Error; err != nil {
		return err
	}
	return nil
}

// Restore un-deletes a soft-deleted record by clearing its soft-delete column.
func (r *GormCRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	meta := r.meta
	softDeleteCol := meta.SoftDeleteColumn
	if softDeleteCol == "" {
		softDeleteCol = "deleted_at"
	}
	return r.db.WithContext(ctx).Unscoped().Model(new(T)).
		Where(clause.Eq{Column: meta.PKColumn, Value: id}).
		Update(softDeleteCol, nil).Error
}

// HardDelete permanently removes a record, bypassing GORM's soft-delete.
func (r *GormCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	meta := r.meta
	return r.db.WithContext(ctx).Unscoped().
		Where(clause.Eq{Column: meta.PKColumn, Value: id}).
		Delete(new(T)).Error
}

// syncAssociations syncs M2M and owned has-many relations after Create/Update.
// Uses direct SQL based on R3 relation metadata — no GORM association tags needed.
func (r *GormCRUD[T, ID]) syncAssociations(ctx context.Context, entityPtr *T) error {
	meta := r.meta
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

// prepareList merges the query args with defaults, resolves any relationship
// ("has") filters against the DB (lowering them into key-set In filters), then
// builds the SQL components. Relationship lowering must happen before clause
// building, which is why this drives the merge/prepare split itself rather than
// calling PrepareListQuery. The relation resolution is skipped entirely when the
// query has no relationship filters, leaving the common path untouched.
func (r *GormCRUD[T, ID]) prepareList(
	ctx context.Context, qarg ...r3.Query,
) (enginesql.PreparedListQuery, error) {
	merged := r.MergeListQuery(qarg...)
	if err := r.schema.ValidateQuery(merged); err != nil {
		return enginesql.PreparedListQuery{}, err
	}
	if hasRelationFilters(merged.Filters) {
		lowered, err := lowerRelationFilters[T](ctx, r.db, merged.Filters)
		if err != nil {
			return enginesql.PreparedListQuery{}, err
		}
		merged.Filters = lowered
	}
	return enginesql.PrepareMergedListQuery(merged)
}

// Raw returns the GormRaw escape hatch for custom queries.
func (r *GormCRUD[T, ID]) Raw() *GormRaw[T, ID] { return r.raw }

// DB returns the underlying *gorm.DB for advanced usage.
func (r *GormCRUD[T, ID]) DB() *gorm.DB { return r.db }
