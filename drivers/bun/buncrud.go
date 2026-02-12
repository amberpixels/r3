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
//     Example: `bun.BaseModel \`bun:"table:cities"\“
//   - Aggregate / custom-shape queries should use Raw().Scan() into a dedicated struct,
//     since Raw().Find() scans into []T and Bun rejects unknown columns.
//   - Bun is the recommended successor to go-pg by the same authors.
package r3bun

import (
	"context"
	"fmt"
	"sync"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/uptrace/bun"
)

// defaults stores the default values for repo queries.
type defaults struct {
	ListQuery r3.Query
	GetQuery  r3.Query
}

// BunCRUD is a CRUD repository based on Bun's *bun.DB.
type BunCRUD[T any, ID comparable] struct {
	db *bun.DB

	defaults   defaults
	defaultsMu sync.RWMutex

	raw *BunRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BunCRUD[any, any]{}

// NewBunCRUD creates a new Bun-based CRUD repository.
func NewBunCRUD[T any, ID comparable](db *bun.DB) *BunCRUD[T, ID] {
	return &BunCRUD[T, ID]{
		db: db,
		defaults: defaults{
			ListQuery: r3.DefaultQuery(),
			GetQuery:  r3.DefaultQuery(),
		},
		raw: NewBunRaw[T, ID](db),
	}
}

// SetDefaultListQuery sets default ListQuery.
func (r *BunCRUD[T, ID]) SetDefaultListQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.ListQuery = q
	r.defaultsMu.Unlock()
}

// SetDefaultGetQuery sets default GetQuery.
func (r *BunCRUD[T, ID]) SetDefaultGetQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.GetQuery = q
	r.defaultsMu.Unlock()
}

func (r *BunCRUD[T, ID]) getDefaultListQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.ListQuery
}

func (r *BunCRUD[T, ID]) getDefaultGetQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.GetQuery
}

func (r *BunCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	_, err := r.db.NewInsert().Model(&entity).Exec(ctx)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *BunCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	var entities []T
	var totalCount int64

	// Merge query with defaults
	q := r.getDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	query := r.db.NewSelect().Model(&entities)

	// Apply preloads (Bun uses Relation for eager loading)
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
		count, err := query.Count(ctx)
		if err != nil {
			return nil, 0, err
		}
		totalCount = int64(count)
		if totalCount == 0 {
			// no reason to call Scan() if it has zero data
			return nil, 0, nil
		}

		// Then add limit & offset for main Scan() query
		limit, offset := q.Pagination.ToLimitOffset()
		query = query.Limit(limit)
		query = query.Offset(offset)
		isPaginated = true
	}

	if err := query.Scan(ctx); err != nil {
		return nil, 0, err
	}

	// If there was no pagination, simply count results
	if !isPaginated {
		totalCount = int64(len(entities))
	}

	return entities, totalCount, nil
}

func (r *BunCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	// Merge query with defaults
	q := r.getDefaultGetQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	query := r.db.NewSelect().Model(&entity).Where("id = ?", id)

	// Apply preloads (Bun uses Relation for eager loading)
	for _, preload := range q.Preloads {
		query = query.Relation(preload.GetName())
	}

	// Handle soft-deleted records
	if q.IncludeTrashed.Some() {
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

func (r *BunCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	_, err := r.db.NewDelete().Model(&entity).Where("id = ?", id).Exec(ctx)
	return err
}

// Raw returns the BunRaw escape hatch for custom queries.
func (r *BunCRUD[T, ID]) Raw() *BunRaw[T, ID] { return r.raw }
