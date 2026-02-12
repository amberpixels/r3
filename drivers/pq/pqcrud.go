// Package r3pq provides an r3.CRUD[T, ID] driver backed by lib/pq,
// the pure Go PostgreSQL driver for database/sql.
//
// Driver: github.com/lib/pq
// Source: https://github.com/lib/pq
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (PqRaw) for arbitrary SQL with $1-style placeholders
//
// Limitations / notes:
//   - No ORM layer: this driver builds raw SQL and uses reflection-based struct scanning.
//     Model structs must use `db` struct tags (e.g. `db:"column_name,pk"`).
//   - No preload support. Relations (joins, eager loading) must be done via Raw().
//   - No soft-delete support. IncludeTrashed is ignored.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - The r3 SQL dialect produces `?` placeholders; this driver converts them to
//     PostgreSQL-style `$1, $2, ...` numbered placeholders internally.
//   - For advanced use cases (transactions, CTEs, etc.), use Raw().DB() to access
//     the underlying *sql.DB directly.
package r3pq

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

// defaults stores the default values for repo queries.
type defaults struct {
	ListQuery r3.Query
	GetQuery  r3.Query
}

// PqCRUD is a CRUD repository based on database/sql with lib/pq.
// Unlike ORM-based drivers, this builds raw SQL queries using the r3 SQL dialect
// and maps struct fields via reflection using `db` struct tags.
type PqCRUD[T any, ID comparable] struct {
	db *sql.DB

	defaults   defaults
	defaultsMu sync.RWMutex

	meta structMeta
	raw  *PqRaw[T, ID]
}

var _ r3.CRUD[any, any] = &PqCRUD[any, any]{}

// NewPqCRUD creates a new database/sql-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewPqCRUD[T any, ID comparable](db *sql.DB) *PqCRUD[T, ID] {
	meta := getStructMeta[T]()
	return &PqCRUD[T, ID]{
		db: db,
		defaults: defaults{
			ListQuery: r3.DefaultQuery(),
			GetQuery:  r3.DefaultQuery(),
		},
		meta: meta,
		raw:  NewPqRaw[T, ID](db),
	}
}

// SetDefaultListQuery sets default ListQuery.
func (r *PqCRUD[T, ID]) SetDefaultListQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.ListQuery = q
	r.defaultsMu.Unlock()
}

// SetDefaultGetQuery sets default GetQuery.
func (r *PqCRUD[T, ID]) SetDefaultGetQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.GetQuery = q
	r.defaultsMu.Unlock()
}

func (r *PqCRUD[T, ID]) getDefaultListQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.ListQuery
}

func (r *PqCRUD[T, ID]) getDefaultGetQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.GetQuery
}

func (r *PqCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	cols := r.meta.nonPKColumns()
	vals := r.meta.nonPKFieldValues(entity)

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		r.meta.tableName,
		columnsString(cols),
		placeholders(len(cols), 1),
		columnsString(r.meta.columns),
	)

	// Scan the returned row back into the entity (includes auto-generated PK)
	dests := r.meta.scanDest(&entity)
	err := r.db.QueryRowContext(ctx, query, vals...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *PqCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	// Merge query with defaults
	q := r.getDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	// Build WHERE clauses
	var whereParts []string
	var whereArgs []any
	argIdx := 1

	clauses, err := r3sql.FiltersToSQL(q.Filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert filters to SQL: %w", err)
	}

	for _, clause := range clauses {
		converted, nextIdx := convertPlaceholders(clause.Clause, argIdx)
		whereParts = append(whereParts, converted)
		whereArgs = append(whereArgs, clause.Args...)
		argIdx = nextIdx
	}

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = " WHERE " + strings.Join(whereParts, " AND ")
	}

	// Build JOIN clauses
	var joinSQL string
	if len(clauses) > 0 {
		var joins []string
		for _, join := range clauses.Joins() {
			joins = append(joins, fmt.Sprintf("JOIN %s ON TRUE", join.String()))
		}
		if len(joins) > 0 {
			joinSQL = " " + strings.Join(joins, " ")
		}
	}

	// Pagination: count first if needed
	var totalCount int64
	var isPaginated bool
	if q.Pagination != nil && q.Pagination.IsPaginated() {
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s%s", r.meta.tableName, joinSQL, whereSQL)
		err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&totalCount)
		if err != nil {
			return nil, 0, err
		}
		if totalCount == 0 {
			return nil, 0, nil
		}
		isPaginated = true
	}

	// Build ORDER BY
	var orderSQL string
	if len(q.Sorts) > 0 {
		sorts, err := r3sql.SortsToSQL(q.Sorts)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to convert sorts to SQL: %w", err)
		}
		var sortParts []string
		for _, sort := range sorts {
			sortParts = append(sortParts, sort.String())
		}
		orderSQL = " ORDER BY " + strings.Join(sortParts, ", ")
	}

	// Build LIMIT/OFFSET
	var limitSQL string
	if isPaginated {
		limit, offset := q.Pagination.ToLimitOffset()
		limitSQL = fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	// Execute the main SELECT
	selectQuery := fmt.Sprintf(
		"SELECT %s FROM %s%s%s%s%s",
		columnsString(r.meta.columns),
		r.meta.tableName,
		joinSQL,
		whereSQL,
		orderSQL,
		limitSQL,
	)

	rows, err := r.db.QueryContext(ctx, selectQuery, whereArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		var entity T
		dests := r.meta.scanDest(&entity)
		if err := rows.Scan(dests...); err != nil {
			return nil, 0, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if !isPaginated {
		totalCount = int64(len(entities))
	}

	return entities, totalCount, nil
}

func (r *PqCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1",
		columnsString(r.meta.columns),
		r.meta.tableName,
		r.meta.pkColumn,
	)

	dests := r.meta.scanDest(&entity)
	err := r.db.QueryRowContext(ctx, query, id).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *PqCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	cols := r.meta.nonPKColumns()
	vals := r.meta.nonPKFieldValues(entity)
	pkVal := r.meta.pkValue(entity)

	// SET col1 = $1, col2 = $2, ... WHERE id = $N
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		r.meta.tableName,
		setExprs(cols, 1),
		r.meta.pkColumn,
		len(cols)+1,
	)

	args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *PqCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = $1",
		r.meta.tableName,
		r.meta.pkColumn,
	)

	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// Raw returns the PqRaw escape hatch for custom queries.
func (r *PqCRUD[T, ID]) Raw() *PqRaw[T, ID] { return r.raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *PqCRUD[T, ID]) DB() *sql.DB { return r.db }

// scanEntities is a helper that scans rows into a slice of T using the struct meta.
func scanEntities[T any](rows *sql.Rows, meta *structMeta) ([]T, error) {
	var entities []T
	for rows.Next() {
		var entity T
		dests := meta.scanDest(&entity)
		if err := rows.Scan(dests...); err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entities, nil
}

// compile-time check that structMeta can produce scan destinations
var _ = reflect.TypeOf(structMeta{})
