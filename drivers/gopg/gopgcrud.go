// Package r3gopg provides an r3.CRUD[T, ID] driver backed by go-pg v10,
// a PostgreSQL ORM with a focus on PostgreSQL-specific features.
//
// Driver: github.com/go-pg/pg/v10
// Source: https://github.com/go-pg/pg
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Preloads via go-pg's Relation() (has-one, has-many, many-to-many)
//   - IncludeTrashed via go-pg's AllWithDeleted() (requires pg:",soft_delete" tag)
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (GoPgRaw) for custom go-pg orm.Query usage
//
// Limitations / notes:
//   - go-pg is in maintenance mode; the authors recommend migrating to Bun.
//     See: https://github.com/go-pg/pg#bun
//   - go-pg does NOT wrap database/sql; it uses its own connection pool (*pg.DB).
//     If you need a *sql.DB (e.g. for goose migrations), open a separate connection.
//   - Model structs must use `pg` struct tags (e.g. `pg:"column_name"`).
//     Table name is set via tableName struct field: `tableName struct{} \`pg:"table_name"\"
//   - Aggregate / custom-shape queries should use Raw().Scan() into a dedicated struct,
//     since Raw().Find() scans into []T and go-pg rejects unknown columns.
package r3gopg

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/sqlbase"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

// GoPgCRUD is a CRUD repository based on go-pg's *pg.DB.
type GoPgCRUD[T any, ID comparable] struct {
	db *pg.DB

	sqlbase.DefaultsManager

	raw *GoPgRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GoPgCRUD[any, any]{}

// NewGoPgCRUD creates a new go-pg-based CRUD repository.
func NewGoPgCRUD[T any, ID comparable](db *pg.DB) *GoPgCRUD[T, ID] {
	return &GoPgCRUD[T, ID]{
		db:              db,
		DefaultsManager: sqlbase.NewDefaultsManager(),
		raw:             NewGoPgRaw[T, ID](db),
	}
}

func (r *GoPgCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	_, err := r.db.ModelContext(ctx, &entity).Insert()
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GoPgCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := sqlbase.PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	var entities []T
	query := r.db.ModelContext(ctx, &entities)

	// Apply preloads (go-pg uses Relation for eager loading)
	for _, preload := range prep.Query.Preloads {
		query = query.Relation(preload.GetName())
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

	entities, totalCount = sqlbase.FinalizeCount(entities, totalCount, prep.IsPaginated)
	return entities, totalCount, nil
}

func (r *GoPgCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	query := r.db.ModelContext(ctx, &entity).Where("id = ?", id)

	// Apply preloads (go-pg uses Relation for eager loading)
	for _, preload := range q.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle soft-deleted records
	if q.IncludeTrashed.Some() {
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

func (r *GoPgCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	_, err := r.db.ModelContext(ctx, &entity).Where("id = ?", id).Delete()
	return err
}

// Raw returns the GoPgRaw escape hatch for custom queries.
func (r *GoPgCRUD[T, ID]) Raw() *GoPgRaw[T, ID] { return r.raw }

// Compile-time check that *orm.Query implements expected interface from go-pg.
var _ orm.DB = (*pg.DB)(nil)
