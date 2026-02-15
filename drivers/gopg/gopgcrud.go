package r3gopg

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

// GoPgCRUD is a CRUD repository based on go-pg's *pg.DB.
// Internally it uses orm.DB so that it can operate within transactions as well.
type GoPgCRUD[T any, ID comparable] struct {
	db   orm.DB
	pgDB *pg.DB // original *pg.DB, nil when inside a transaction
	enginesql.DefaultsManager

	raw *GoPgRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GoPgCRUD[any, any]{}

// NewGoPgCRUD creates a new go-pg-based CRUD repository.
func NewGoPgCRUD[T any, ID comparable](db *pg.DB) *GoPgCRUD[T, ID] {
	return &GoPgCRUD[T, ID]{
		db:              db,
		pgDB:            db,
		DefaultsManager: enginesql.NewDefaultsManager(),
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
	if fieldCols := enginesql.FieldsToColumns(prep.Query.Fields); len(fieldCols) > 0 {
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

	entities, totalCount = enginesql.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

func (r *GoPgCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	query := r.db.ModelContext(ctx, &entity).Where("id = ?", id)

	// Apply fields selection
	if fieldCols := enginesql.FieldsToColumns(q.Fields); len(fieldCols) > 0 {
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
	cols := enginesql.FieldsToColumns(fields)

	meta := enginesql.GetStructMeta[T]()
	cols, err := meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}

	_, err = r.db.ModelContext(ctx, &entity).Column(cols...).WherePK().Update()
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GoPgCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	_, err := r.db.ModelContext(ctx, &entity).Where("id = ?", id).Delete()
	return err
}

// Raw returns the GoPgRaw escape hatch for custom queries.
func (r *GoPgCRUD[T, ID]) Raw() *GoPgRaw[T, ID] { return r.raw }

// Compile-time check that *orm.Query implements expected interface from go-pg.
var _ orm.DB = (*pg.DB)(nil)
