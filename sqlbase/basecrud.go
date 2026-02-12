package sqlbase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

// Defaults stores the default values for repo queries.
type Defaults struct {
	ListQuery r3.Query
	GetQuery  r3.Query
}

// BaseCRUD is a generic CRUD repository backed by database/sql.
// It implements the full r3.CRUD[T, ID] interface using reflection-based struct scanning
// and the r3 SQL dialect for filters, sorts, and pagination.
//
// Database-specific drivers embed this type and may override individual methods
// (e.g. MySQL overrides Create because it lacks RETURNING support).
type BaseCRUD[T any, ID comparable] struct {
	DB *sql.DB

	Defaults   Defaults
	DefaultsMu sync.RWMutex

	Meta   StructMeta
	Flavor Flavor
	Raw    *BaseRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BaseCRUD[any, any]{}

// NewBaseCRUD creates a new BaseCRUD with the given database connection and flavor.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
func NewBaseCRUD[T any, ID comparable](db *sql.DB, flavor Flavor) *BaseCRUD[T, ID] {
	meta := GetStructMeta[T]()
	return &BaseCRUD[T, ID]{
		DB: db,
		Defaults: Defaults{
			ListQuery: r3.DefaultQuery(),
			GetQuery:  r3.DefaultQuery(),
		},
		Meta:   meta,
		Flavor: flavor,
		Raw:    NewBaseRaw[T, ID](db, meta),
	}
}

// SetDefaultListQuery sets default ListQuery.
func (r *BaseCRUD[T, ID]) SetDefaultListQuery(q r3.Query) {
	r.DefaultsMu.Lock()
	r.Defaults.ListQuery = q
	r.DefaultsMu.Unlock()
}

// SetDefaultGetQuery sets default GetQuery.
func (r *BaseCRUD[T, ID]) SetDefaultGetQuery(q r3.Query) {
	r.DefaultsMu.Lock()
	r.Defaults.GetQuery = q
	r.DefaultsMu.Unlock()
}

// GetDefaultListQuery returns the default ListQuery (thread-safe).
func (r *BaseCRUD[T, ID]) GetDefaultListQuery() r3.Query {
	r.DefaultsMu.RLock()
	defer r.DefaultsMu.RUnlock()
	return r.Defaults.ListQuery
}

// GetDefaultGetQuery returns the default GetQuery (thread-safe).
func (r *BaseCRUD[T, ID]) GetDefaultGetQuery() r3.Query {
	r.DefaultsMu.RLock()
	defer r.DefaultsMu.RUnlock()
	return r.Defaults.GetQuery
}

// Create inserts a new record and returns it with the auto-generated PK populated.
// This implementation uses RETURNING and works for PostgreSQL and SQLite 3.35+.
// MySQL drivers should override this method.
func (r *BaseCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if !r.Flavor.SupportsRETURNING {
		return r.createWithLastInsertID(ctx, entity)
	}

	cols := r.Meta.NonPKColumns()
	vals := r.Meta.NonPKFieldValues(entity)

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		r.Meta.TableName,
		ColumnsString(cols),
		r.Flavor.Placeholders(len(cols), 1),
		ColumnsString(r.Meta.Columns),
	)

	dests := r.Meta.ScanDest(&entity)
	err := r.DB.QueryRowContext(ctx, query, vals...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// createWithLastInsertID is the fallback Create for databases without RETURNING (MySQL).
func (r *BaseCRUD[T, ID]) createWithLastInsertID(ctx context.Context, entity T) (T, error) {
	cols := r.Meta.NonPKColumns()
	vals := r.Meta.NonPKFieldValues(entity)

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		r.Meta.TableName,
		ColumnsString(cols),
		r.Flavor.Placeholders(len(cols), 1),
	)

	result, err := r.DB.ExecContext(ctx, insertQuery, vals...)
	if err != nil {
		return entity, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return entity, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	r.Meta.SetPKValue(&entity, lastID)

	selectQuery := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		ColumnsString(r.Meta.Columns),
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)

	dests := r.Meta.ScanDest(&entity)
	err = r.DB.QueryRowContext(ctx, selectQuery, lastID).Scan(dests...)
	if err != nil {
		return entity, err
	}

	return entity, nil
}

// List retrieves records based on the provided query parameters.
func (r *BaseCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	q := r.GetDefaultListQuery()
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
		converted, nextIdx := r.Flavor.ConvertPlaceholders(clause.Clause, argIdx)
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
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s%s", r.Meta.TableName, joinSQL, whereSQL)
		err := r.DB.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&totalCount)
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
		ColumnsString(r.Meta.Columns),
		r.Meta.TableName,
		joinSQL,
		whereSQL,
		orderSQL,
		limitSQL,
	)

	rows, err := r.DB.QueryContext(ctx, selectQuery, whereArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		var entity T
		dests := r.Meta.ScanDest(&entity)
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

// Get retrieves a record by its ID.
func (r *BaseCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		ColumnsString(r.Meta.Columns),
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)

	dests := r.Meta.ScanDest(&entity)
	err := r.DB.QueryRowContext(ctx, query, id).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Update modifies an existing record identified by its primary key.
func (r *BaseCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	cols := r.Meta.NonPKColumns()
	vals := r.Meta.NonPKFieldValues(entity)
	pkVal := r.Meta.PKValue(entity)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.SetExprs(cols, 1),
		r.Flavor.WhereEq(r.Meta.PKColumn, len(cols)+1),
	)

	args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
	_, err := r.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Delete removes a record by its ID.
func (r *BaseCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)

	_, err := r.DB.ExecContext(ctx, query, id)
	return err
}

// ScanEntities scans sql.Rows into a slice of T using the given StructMeta.
func ScanEntities[T any](rows *sql.Rows, meta *StructMeta) ([]T, error) {
	var entities []T
	for rows.Next() {
		var entity T
		dests := meta.ScanDest(&entity)
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
