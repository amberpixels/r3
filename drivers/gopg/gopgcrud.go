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
//     Table name is set via tableName struct field: `tableName struct{} \`pg:"table_name"\“
//   - Aggregate / custom-shape queries should use Raw().Scan() into a dedicated struct,
//     since Raw().Find() scans into []T and go-pg rejects unknown columns.
package r3gopg

import (
	"context"
	"fmt"
	"sync"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

// defaults stores the default values for repo queries.
type defaults struct {
	ListQuery r3.Query
	GetQuery  r3.Query
}

// GoPgCRUD is a CRUD repository based on go-pg's *pg.DB.
type GoPgCRUD[T any, ID comparable] struct {
	db *pg.DB

	defaults   defaults
	defaultsMu sync.RWMutex

	raw *GoPgRaw[T, ID]
}

var _ r3.CRUD[any, any] = &GoPgCRUD[any, any]{}

// NewGoPgCRUD creates a new go-pg-based CRUD repository.
func NewGoPgCRUD[T any, ID comparable](db *pg.DB) *GoPgCRUD[T, ID] {
	return &GoPgCRUD[T, ID]{
		db: db,
		defaults: defaults{
			ListQuery: r3.DefaultQuery(),
			GetQuery:  r3.DefaultQuery(),
		},
		raw: NewGoPgRaw[T, ID](db),
	}
}

// SetDefaultListQuery sets default ListQuery.
func (r *GoPgCRUD[T, ID]) SetDefaultListQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.ListQuery = q
	r.defaultsMu.Unlock()
}

// SetDefaultGetQuery sets default GetQuery.
func (r *GoPgCRUD[T, ID]) SetDefaultGetQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.GetQuery = q
	r.defaultsMu.Unlock()
}

func (r *GoPgCRUD[T, ID]) getDefaultListQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.ListQuery
}

func (r *GoPgCRUD[T, ID]) getDefaultGetQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.GetQuery
}

func (r *GoPgCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	_, err := r.db.ModelContext(ctx, &entity).Insert()
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *GoPgCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	var entities []T
	var totalCount int64

	// Merge query with defaults
	q := r.getDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	query := r.db.ModelContext(ctx, &entities)

	// Apply preloads (go-pg uses Relation for eager loading)
	for _, preload := range q.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle custom filters
	clauses, err := r3sql.FiltersToSQL(q.Filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert filters to SQL: %w", err)
	}

	if len(clauses) > 0 {
		for _, join := range clauses.Joins() {
			query = query.Join(fmt.Sprintf("JOIN %s ON TRUE", join.String()))
		}

		for _, clause := range clauses {
			query = query.Where(clause.Clause, clause.Args...)
		}
	}

	// Sorting
	if len(q.Sorts) > 0 {
		sorts, err := r3sql.SortsToSQL(q.Sorts)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to convert sorts to SQL: %w", err)
		}

		for _, sort := range sorts {
			query = query.OrderExpr(sort.String())
		}
	}

	// If Pagination is given, then we need to first Count all results without pagination:
	var isPaginated bool
	if q.Pagination != nil && q.Pagination.IsPaginated() {
		// We have to count first:
		count, err := query.Count()
		if err != nil {
			return nil, 0, err
		}
		totalCount = int64(count)
		if totalCount == 0 {
			// no reason to call Select() if it has zero data
			return nil, 0, nil
		}

		// Then add limit & offset for main Select() query
		limit, offset := q.Pagination.ToLimitOffset()
		query = query.Limit(limit)
		query = query.Offset(offset)
		isPaginated = true
	}

	if err := query.Select(); err != nil {
		return nil, 0, err
	}

	// If there was no pagination, simply count results
	if !isPaginated {
		totalCount = int64(len(entities))
	}

	return entities, totalCount, nil
}

func (r *GoPgCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	// Merge query with defaults
	q := r.getDefaultGetQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

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
