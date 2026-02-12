// Package r3mysql provides an r3.CRUD[T, ID] driver backed by go-sql-driver/mysql,
// the pure Go MySQL driver for database/sql.
//
// Driver: github.com/go-sql-driver/mysql
// Source: https://github.com/go-sql-driver/mysql
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (MysqlRaw) for arbitrary SQL with ?-style placeholders
//
// Limitations / notes:
//   - No ORM layer: this driver builds raw SQL and uses reflection-based struct scanning.
//     Model structs must use `db` struct tags (e.g. `db:"column_name,pk"`).
//   - No preload support. Relations (joins, eager loading) must be done via Raw().
//   - No soft-delete support. IncludeTrashed is ignored.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - Uses `?` placeholders natively (no conversion needed).
//   - MySQL does not support RETURNING; Create uses INSERT + LAST_INSERT_ID() + SELECT
//     to return the full created entity including auto-generated columns.
//   - ILIKE is not supported by MySQL; use LIKE instead (case sensitivity depends on
//     column collation; default utf8mb4_0900_ai_ci is case-insensitive).
//   - NULLS FIRST/LAST is not natively supported by MySQL.
//   - DSN must include `parseTime=true` for time.Time scanning to work correctly.
//   - For advanced use cases (transactions, CTEs, etc.), use Raw().DB() to access
//     the underlying *sql.DB directly.
package r3mysql

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

// MysqlCRUD is a CRUD repository based on database/sql with go-sql-driver/mysql.
// Unlike ORM-based drivers, this builds raw SQL queries using the r3 SQL dialect
// and maps struct fields via reflection using `db` struct tags.
type MysqlCRUD[T any, ID comparable] struct {
	db *sql.DB

	defaults   defaults
	defaultsMu sync.RWMutex

	meta structMeta
	raw  *MysqlRaw[T, ID]
}

var _ r3.CRUD[any, any] = &MysqlCRUD[any, any]{}

// NewMysqlCRUD creates a new MySQL-based CRUD repository.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewMysqlCRUD[T any, ID comparable](db *sql.DB) *MysqlCRUD[T, ID] {
	meta := getStructMeta[T]()
	return &MysqlCRUD[T, ID]{
		db: db,
		defaults: defaults{
			ListQuery: r3.DefaultQuery(),
			GetQuery:  r3.DefaultQuery(),
		},
		meta: meta,
		raw:  NewMysqlRaw[T, ID](db),
	}
}

// SetDefaultListQuery sets default ListQuery.
func (r *MysqlCRUD[T, ID]) SetDefaultListQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.ListQuery = q
	r.defaultsMu.Unlock()
}

// SetDefaultGetQuery sets default GetQuery.
func (r *MysqlCRUD[T, ID]) SetDefaultGetQuery(q r3.Query) {
	r.defaultsMu.Lock()
	r.defaults.GetQuery = q
	r.defaultsMu.Unlock()
}

func (r *MysqlCRUD[T, ID]) getDefaultListQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.ListQuery
}

func (r *MysqlCRUD[T, ID]) getDefaultGetQuery() r3.Query {
	r.defaultsMu.RLock()
	defer r.defaultsMu.RUnlock()
	return r.defaults.GetQuery
}

func (r *MysqlCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	cols := r.meta.nonPKColumns()
	vals := r.meta.nonPKFieldValues(entity)

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		r.meta.tableName,
		columnsString(cols),
		placeholders(len(cols)),
	)

	// MySQL does not support RETURNING; use INSERT + LAST_INSERT_ID() + SELECT.
	result, err := r.db.ExecContext(ctx, insertQuery, vals...)
	if err != nil {
		return entity, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return entity, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// Set the PK on the entity
	r.meta.setPKValue(&entity, lastID)

	// Re-fetch to get all column values (including defaults, triggers, etc.)
	selectQuery := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = ?",
		columnsString(r.meta.columns),
		r.meta.tableName,
		r.meta.pkColumn,
	)

	dests := r.meta.scanDest(&entity)
	err = r.db.QueryRowContext(ctx, selectQuery, lastID).Scan(dests...)
	if err != nil {
		return entity, err
	}

	return entity, nil
}

func (r *MysqlCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	// Merge query with defaults
	q := r.getDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}

	// Build WHERE clauses
	var whereParts []string
	var whereArgs []any

	clauses, err := r3sql.FiltersToSQL(q.Filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert filters to SQL: %w", err)
	}

	for _, clause := range clauses {
		// MySQL uses ? placeholders natively, same as the r3 SQL dialect output.
		// No placeholder conversion needed.
		whereParts = append(whereParts, clause.Clause)
		whereArgs = append(whereArgs, clause.Args...)
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

func (r *MysqlCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = ?",
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

func (r *MysqlCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	cols := r.meta.nonPKColumns()
	vals := r.meta.nonPKFieldValues(entity)
	pkVal := r.meta.pkValue(entity)

	// SET col1 = ?, col2 = ?, ... WHERE id = ?
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		r.meta.tableName,
		setExprs(cols),
		r.meta.pkColumn,
	)

	args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

func (r *MysqlCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = ?",
		r.meta.tableName,
		r.meta.pkColumn,
	)

	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// Raw returns the MysqlRaw escape hatch for custom queries.
func (r *MysqlCRUD[T, ID]) Raw() *MysqlRaw[T, ID] { return r.raw }

// DB returns the underlying *sql.DB for advanced usage.
func (r *MysqlCRUD[T, ID]) DB() *sql.DB { return r.db }

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
