// Package r3bun provides an r3.CRUD[T, ID] driver backed by Bun,
// a SQL-first Go ORM for PostgreSQL, MySQL, MSSQL, and SQLite.
//
// Driver: github.com/uptrace/bun
// Source: https://github.com/uptrace/bun
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Preloads via Bun's Relation() (belongs-to, has-one, has-many, many-to-many)
//   - IncludeTrashed via Bun's WhereAllWithDeleted() (requires soft-delete model setup)
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (BunRaw) for custom bun.SelectQuery usage
//
// Limitations / notes:
//   - Bun wraps database/sql natively; use db.DB (the underlying *sql.DB) for goose
//     migrations or any raw database/sql usage.
//   - Model structs must embed bun.BaseModel with a table tag and use `bun` struct tags.
//     Example: `bun.BaseModel \`bun:"table:cities"\"
//   - Aggregate / custom-shape queries should use Raw().Scan() into a dedicated struct,
//     since Raw().Find() scans into []T and Bun rejects unknown columns.
//   - Bun is the recommended successor to go-pg by the same authors.
package r3bun

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/sqlbase"
	"github.com/uptrace/bun"
)

// BunCRUD is a CRUD repository based on Bun's *bun.DB.
// Internally it uses bun.IDB so that it can operate within transactions as well.
type BunCRUD[T any, ID comparable] struct {
	db    bun.IDB
	sqlDB *bun.DB // original *bun.DB, nil when inside a transaction

	sqlbase.DefaultsManager

	raw *BunRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BunCRUD[any, any]{}

// NewBunCRUD creates a new Bun-based CRUD repository.
func NewBunCRUD[T any, ID comparable](db *bun.DB) *BunCRUD[T, ID] {
	return &BunCRUD[T, ID]{
		db:              db,
		sqlDB:           db,
		DefaultsManager: sqlbase.NewDefaultsManager(),
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
	prep, err := sqlbase.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	var entities []T
	query := r.db.NewSelect().Model(&entities)

	// Apply fields selection
	if fieldCols := sqlbase.FieldsToColumns(prep.Query.Fields); len(fieldCols) > 0 {
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

	entities, totalCount = sqlbase.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

func (r *BunCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	query := r.db.NewSelect().Model(&entity).Where("id = ?", id)

	// Apply fields selection
	if fieldCols := sqlbase.FieldsToColumns(q.Fields); len(fieldCols) > 0 {
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
	cols := sqlbase.FieldsToColumns(fields)

	meta := sqlbase.GetStructMeta[T]()
	cols, err := meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}

	_, err = r.db.NewUpdate().Model(&entity).Column(cols...).WherePK().Exec(ctx)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *BunCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	_, err := r.db.NewDelete().Model(&entity).Where("id = ?", id).Exec(ctx)
	return err
}

// Raw returns the BunRaw escape hatch for custom queries.
func (r *BunCRUD[T, ID]) Raw() *BunRaw[T, ID] { return r.raw }
