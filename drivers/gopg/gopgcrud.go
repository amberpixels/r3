package r3gopg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/go-pg/pg/v10/types"
)

// GoPgCRUD is a CRUD repository based on go-pg's *pg.DB.
// Internally it uses orm.DB so that it can operate within transactions as well.
type GoPgCRUD[T any, ID comparable] struct {
	db   orm.DB
	pgDB *pg.DB // original *pg.DB, nil when inside a transaction
	enginesql.DefaultsManager

	Config r3.Config
	raw    *GoPgRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GoPgCRUD[any, any]{}
var _ r3.Aggregator = &GoPgCRUD[any, any]{}

// NewGoPgCRUD creates a new go-pg-based CRUD repository.
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewGoPgCRUD[T any, ID comparable](db *pg.DB, opts ...r3.Option) *GoPgCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	// Value codecs are not wired into the go-pg driver yet; reject a declared
	// codec loudly rather than binding the un-encoded value.
	r3.RequireCodecSupport(r3.SchemaOf[T](r3.WithSchemaNaming(resolved.Config.Naming)), "r3/gopg")
	return &GoPgCRUD[T, ID]{
		db:              db,
		pgDB:            db,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Config:          resolved.Config,
		raw:             NewGoPgRaw[T, ID](db),
	}
}

// PgDB returns the underlying *pg.DB for advanced usage.
// Panics if GoPgCRUD is operating inside a transaction.
func (r *GoPgCRUD[T, ID]) PgDB() *pg.DB {
	if r.pgDB == nil {
		panic("r3/gopg: PgDB() called on a transactional GoPgCRUD (use the transaction's CRUD instead)")
	}
	return r.pgDB
}

func (r *GoPgCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	_, err := r.db.ModelContext(ctx, &entity).Insert()
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GoPgCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := enginesql.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	var entities []T
	query := r.db.ModelContext(ctx, &entities)

	// Apply fields selection
	if fieldCols := r3.FieldsToStrings(prep.Query.Fields); len(fieldCols) > 0 {
		query = query.Column(fieldCols...)
	}

	// Apply preloads (go-pg uses Relation for eager loading)
	for _, preload := range prep.Query.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle soft-deleted records
	if prep.Query.IncludeTrashed.Some(true) {
		query = query.AllWithDeleted()
	}

	// Apply joins
	for _, join := range prep.Joins() {
		query = query.Join(fmt.Sprintf("JOIN %s ON TRUE", join.String()))
	}

	// Apply filters
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	// Apply sorts
	for _, sort := range prep.Sorts {
		query = query.OrderExpr(sort.String())
	}

	// Pagination: count first, then limit/offset
	var totalCount int64
	if prep.IsPaginated {
		count, err := query.Count()
		if err != nil {
			return nil, 0, err
		}
		totalCount = int64(count)
		if totalCount == 0 {
			return nil, 0, nil
		}
		query = query.Limit(prep.Limit).Offset(prep.Offset)
	}

	if err := query.Select(); err != nil {
		return nil, 0, err
	}

	entities, totalCount = r3.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

// Count returns the number of records matching the query's filters.
func (r *GoPgCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	prep, err := enginesql.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return 0, err
	}

	var entities []T
	query := r.db.ModelContext(ctx, &entities)

	if prep.Query.IncludeTrashed.Some(true) {
		query = query.AllWithDeleted()
	}
	for _, join := range prep.Joins() {
		query = query.Join(fmt.Sprintf("JOIN %s ON TRUE", join.String()))
	}
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	count, err := query.Count()
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// Aggregate computes grouped aggregates over the records matching the query.
// See r3.Aggregator for the query semantics.
func (r *GoPgCRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	// go-pg repos carry no r3.Schema; a zero schema still validates the
	// aggregate structure.
	prep, err := enginesql.PrepareAggregateQuery(&r.DefaultsManager, r3.Schema{}, qarg...)
	if err != nil {
		return nil, err
	}

	var entities []T
	query := r.db.ModelContext(ctx, &entities)

	if prep.Query.IncludeTrashed.Some(true) {
		query = query.AllWithDeleted()
	}
	for _, join := range prep.Joins() {
		query = query.Join(fmt.Sprintf("JOIN %s ON TRUE", join.String()))
	}
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	for _, item := range prep.SelectList {
		query = query.ColumnExpr(item)
	}
	if len(prep.GroupBy) > 0 {
		query = query.GroupExpr(strings.Join(prep.GroupBy, ", "))
	}
	if prep.Having.Clause != "" {
		query = query.Having(prep.Having.Clause, prep.Having.Args...)
	}
	for _, sort := range prep.Sorts {
		query = query.OrderExpr(sort.String())
	}
	if prep.IsPaginated {
		query = query.Limit(prep.Limit).Offset(prep.Offset)
	}

	// go-pg scans dynamic columns natively into a map slice.
	var results []map[string]any
	if err := query.Select(&results); err != nil {
		return nil, err
	}

	rows := make([]r3.AggregateRow, len(results))
	for i, m := range results {
		row := make(r3.AggregateRow, len(m))
		for k, v := range m {
			switch tv := v.(type) {
			case []byte:
				v = string(tv)
			case types.RawValue:
				// go-pg wraps types it has no Go mapping for (e.g. NUMERIC,
				// which Postgres returns for SUM/AVG) in a RawValue; its
				// textual form is what the AggregateRow accessors coerce.
				v = tv.Value
			}
			row[k] = v
		}
		rows[i] = row
	}
	return rows, nil
}

func (r *GoPgCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	meta := enginesql.GetStructMeta[T]()
	query := r.db.ModelContext(ctx, &entity).Where("? = ?", pg.Ident(meta.PKColumn), id)

	// Apply fields selection
	if fieldCols := r3.FieldsToStrings(q.Fields); len(fieldCols) > 0 {
		query = query.Column(fieldCols...)
	}

	// Apply preloads (go-pg uses Relation for eager loading)
	for _, preload := range q.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle soft-deleted records
	if q.IncludeTrashed.Some(true) {
		query = query.AllWithDeleted()
	}

	if err := query.Select(); err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return entity, r3.ErrNotFound
		}
		return entity, err
	}
	return entity, nil
}

func (r *GoPgCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	_, err := r.db.ModelContext(ctx, &entity).WherePK().Update()
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GoPgCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	cols := r3.FieldsToStrings(fields)

	meta := enginesql.GetStructMeta[T]()
	cols, err := meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}

	_, err = r.db.ModelContext(ctx, &entity).Column(cols...).WherePK().Update()
	if err != nil {
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

func (r *GoPgCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	meta := enginesql.GetStructMeta[T]()
	_, err := r.db.ModelContext(ctx, &entity).
		Where("? = ?", pg.Ident(meta.PKColumn), id).Delete()
	return err
}

// Restore un-deletes a soft-deleted record by clearing its soft-delete column.
func (r *GoPgCRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	meta := enginesql.GetStructMeta[T]()
	_, err := r.db.ModelContext(ctx, (*T)(nil)).
		Set("? = NULL", pg.Ident(softDeleteColumn(meta))).
		Where("? = ?", pg.Ident(meta.PKColumn), id).
		AllWithDeleted().Update()
	return err
}

// HardDelete permanently removes a record, bypassing go-pg's soft-delete.
func (r *GoPgCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	meta := enginesql.GetStructMeta[T]()
	_, err := r.db.ModelContext(ctx, (*T)(nil)).
		Where("? = ?", pg.Ident(meta.PKColumn), id).AllWithDeleted().ForceDelete()
	return err
}

// softDeleteColumn resolves the soft-delete column from struct metadata, falling
// back to "deleted_at" when the model declares no r3 soft-delete tag (e.g. it
// relies on the ORM's native soft-delete column).
func softDeleteColumn(meta enginesql.StructMeta) string {
	if meta.SoftDeleteColumn != "" {
		return meta.SoftDeleteColumn
	}
	return "deleted_at"
}

// NewGoPgQuerier creates a read-only go-pg-based repository.
// Returns [r3.Querier] — a compile-time guarantee of read-only access.
func NewGoPgQuerier[T any, ID comparable](db *pg.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewGoPgCRUD[T, ID](db, opts...)
}

// Raw returns the GoPgRaw escape hatch for custom queries.
func (r *GoPgCRUD[T, ID]) Raw() *GoPgRaw[T, ID] { return r.raw }

// Compile-time check that *orm.Query implements expected interface from go-pg.
var _ orm.DB = (*pg.DB)(nil)
