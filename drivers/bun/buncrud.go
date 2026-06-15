package r3bun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"github.com/uptrace/bun"
)

// BunCRUD is a CRUD repository based on Bun's *bun.DB.
// Internally it uses bun.IDB so that it can operate within transactions as well.
type BunCRUD[T any, ID comparable] struct {
	db    bun.IDB
	sqlDB *bun.DB // original *bun.DB, nil when inside a transaction

	enginesql.DefaultsManager

	Config r3.Config
	raw    *BunRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BunCRUD[any, any]{}

// NewBunCRUD creates a new Bun-based CRUD repository.
//
// Accepts optional [r3.Option] values for framework-level configuration.
func NewBunCRUD[T any, ID comparable](db *bun.DB, opts ...r3.Option) *BunCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	return &BunCRUD[T, ID]{
		db:              db,
		sqlDB:           db,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Config:          resolved.Config,
		raw:             NewBunRaw[T, ID](db),
	}
}

// DB returns the underlying *bun.DB for advanced usage.
// Panics if BunCRUD is operating inside a transaction.
func (r *BunCRUD[T, ID]) DB() *bun.DB {
	if r.sqlDB == nil {
		panic("r3/bun: DB() called on a transactional BunCRUD (use the transaction's CRUD instead)")
	}
	return r.sqlDB
}

func (r *BunCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	_, err := r.db.NewInsert().Model(&entity).Exec(ctx)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *BunCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := enginesql.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	var entities []T
	query := r.db.NewSelect().Model(&entities)

	// Apply fields selection
	if fieldCols := r3.FieldsToStrings(prep.Query.Fields); len(fieldCols) > 0 {
		query = query.Column(fieldCols...)
	}

	// Apply preloads (Bun uses Relation for eager loading)
	for _, preload := range prep.Query.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle soft-deleted records
	if prep.Query.IncludeTrashed.Some(true) {
		query = query.WhereAllWithDeleted()
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
		count, err := query.Count(ctx)
		if err != nil {
			return nil, 0, err
		}
		totalCount = int64(count)
		if totalCount == 0 {
			return nil, 0, nil
		}
		query = query.Limit(prep.Limit).Offset(prep.Offset)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, 0, err
	}

	entities, totalCount = r3.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

// Count returns the number of records matching the query's filters.
func (r *BunCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	prep, err := enginesql.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return 0, err
	}

	var entities []T
	query := r.db.NewSelect().Model(&entities)

	if prep.Query.IncludeTrashed.Some(true) {
		query = query.WhereAllWithDeleted()
	}
	for _, join := range prep.Joins() {
		query = query.Join(fmt.Sprintf("JOIN %s ON TRUE", join.String()))
	}
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *BunCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	query := r.db.NewSelect().Model(&entity).Where("id = ?", id)

	// Apply fields selection
	if fieldCols := r3.FieldsToStrings(q.Fields); len(fieldCols) > 0 {
		query = query.Column(fieldCols...)
	}

	// Apply preloads (Bun uses Relation for eager loading)
	for _, preload := range q.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle soft-deleted records
	if q.IncludeTrashed.Some(true) {
		query = query.WhereAllWithDeleted()
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity, r3.ErrNotFound
		}
		return entity, err
	}
	return entity, nil
}

func (r *BunCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	_, err := r.db.NewUpdate().Model(&entity).WherePK().Exec(ctx)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *BunCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	cols := r3.FieldsToStrings(fields)

	meta := enginesql.GetStructMeta[T]()
	cols, err := meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}

	_, err = r.db.NewUpdate().Model(&entity).Column(cols...).WherePK().Exec(ctx)
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

func (r *BunCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	_, err := r.db.NewDelete().Model(&entity).Where("id = ?", id).Exec(ctx)
	return err
}

// Restore un-deletes a soft-deleted record by clearing its deleted_at field.
func (r *BunCRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	_, err := r.db.NewUpdate().Model((*T)(nil)).
		Set("deleted_at = NULL").Where("id = ?", id).
		WhereAllWithDeleted().Exec(ctx)
	return err
}

// HardDelete permanently removes a record, bypassing Bun's soft-delete.
func (r *BunCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	_, err := r.db.NewDelete().Model((*T)(nil)).
		Where("id = ?", id).ForceDelete().Exec(ctx)
	return err
}

// NewBunQuerier creates a read-only Bun-based repository.
// Returns [r3.Querier] — a compile-time guarantee of read-only access.
func NewBunQuerier[T any, ID comparable](db *bun.DB, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBunCRUD[T, ID](db, opts...)
}

// Raw returns the BunRaw escape hatch for custom queries.
func (r *BunCRUD[T, ID]) Raw() *BunRaw[T, ID] { return r.raw }
