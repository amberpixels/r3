package enginesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// SQLExecutor is the common interface between *sql.DB and *sql.Tx.
// Both types satisfy this interface, allowing BaseCRUD to operate
// transparently in both normal and transactional modes.
type SQLExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// BaseCRUD is a generic CRUD repository backed by database/sql.
// It implements the full r3.CRUD[T, ID] interface using reflection-based struct scanning
// and the r3 SQL dialect for filters, sorts, and pagination.
//
// Database-specific drivers embed this type and may override individual methods
// (e.g. MySQL overrides Create because it lacks RETURNING support).
type BaseCRUD[T any, ID comparable] struct {
	// Executor is the SQL executor used for all queries. It is either a *sql.DB
	// or a *sql.Tx, both of which implement [SQLExecutor].
	Executor SQLExecutor

	// sqlDB holds the original *sql.DB, used for starting transactions.
	// It is nil when BaseCRUD is operating inside a transaction.
	sqlDB *sql.DB

	DefaultsManager

	Meta   StructMeta
	Flavor Flavor
	Config r3.Config
	Raw    *BaseRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BaseCRUD[any, any]{}

// NewBaseCRUD creates a new BaseCRUD with the given database connection and flavor.
// Models should use `db:"column_name"` struct tags for column mapping.
// The primary key field should be tagged with `db:"id,pk"` (defaults to "id").
//
// Accepts optional [r3.Option] values for framework-level configuration
// (e.g. [r3.WithConfig] for custom naming conventions or default page size).
func NewBaseCRUD[T any, ID comparable](db *sql.DB, flavor Flavor, opts ...r3.Option) *BaseCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	meta := GetStructMeta[T]()
	return &BaseCRUD[T, ID]{
		Executor:        db,
		sqlDB:           db,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Flavor:          flavor,
		Config:          resolved.Config,
		Raw:             NewBaseRaw[T, ID](db, meta),
	}
}

// SQLDB returns the underlying *sql.DB for advanced usage (opening new connections,
// starting transactions manually, etc.).
// Panics if BaseCRUD is operating inside a transaction (use the transaction's CRUD instead).
func (r *BaseCRUD[T, ID]) SQLDB() *sql.DB {
	if r.sqlDB == nil {
		panic("r3/engine/sql: SQLDB() called on a transactional BaseCRUD (use the transaction's CRUD instead)")
	}
	return r.sqlDB
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
	err := r.Executor.QueryRowContext(ctx, query, vals...).Scan(dests...)
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

	result, err := r.Executor.ExecContext(ctx, insertQuery, vals...)
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
	err = r.Executor.QueryRowContext(ctx, selectQuery, lastID).Scan(dests...)
	if err != nil {
		return entity, err
	}

	return entity, nil
}

// List retrieves records based on the provided query parameters.
func (r *BaseCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	// Build WHERE clauses (with placeholder conversion for this flavor)
	var whereParts []string
	var whereArgs []any
	argIdx := 1

	// Soft-delete: auto-add WHERE deleted_at IS NULL unless IncludeTrashed is true
	if r.Meta.SoftDeleteColumn != "" && !prep.Query.IncludeTrashed.Some(true) {
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", r.Meta.SoftDeleteColumn))
	}

	for _, clause := range prep.Clauses {
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
	if joins := prep.Joins(); len(joins) > 0 {
		var joinParts []string
		for _, join := range joins {
			joinParts = append(joinParts, fmt.Sprintf("JOIN %s ON TRUE", join.String()))
		}
		joinSQL = " " + strings.Join(joinParts, " ")
	}

	// Pagination: count first if needed
	var totalCount int64
	if prep.IsPaginated {
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s%s", r.Meta.TableName, joinSQL, whereSQL)
		err := r.Executor.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&totalCount)
		if err != nil {
			return nil, 0, err
		}
		if totalCount == 0 {
			return nil, 0, nil
		}
	}

	// Build ORDER BY
	var orderSQL string
	if len(prep.Sorts) > 0 {
		var sortParts []string
		for _, sort := range prep.Sorts {
			sortParts = append(sortParts, sort.String())
		}
		orderSQL = " ORDER BY " + strings.Join(sortParts, ", ")
	}

	// Build LIMIT/OFFSET
	var limitSQL string
	if prep.IsPaginated {
		limitSQL = fmt.Sprintf(" LIMIT %d OFFSET %d", prep.Limit, prep.Offset)
	}

	// Resolve selected columns (Fields selection)
	selectedCols := FieldsToColumns(prep.Query.Fields)
	selectCols, _ := r.Meta.FieldIndicesForColumns(selectedCols)

	// Execute the main SELECT
	selectQuery := fmt.Sprintf(
		"SELECT %s FROM %s%s%s%s%s",
		ColumnsString(selectCols),
		r.Meta.TableName,
		joinSQL,
		whereSQL,
		orderSQL,
		limitSQL,
	)

	rows, err := r.Executor.QueryContext(ctx, selectQuery, whereArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		var entity T
		dests := r.Meta.ScanDestForColumns(&entity, selectedCols)
		if err := rows.Scan(dests...); err != nil {
			return nil, 0, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	entities, totalCount = FinalizeCount(entities, totalCount, prep.IsPaginated)

	// Run preloads if any are requested and the model has relations
	if len(prep.Query.Preloads) > 0 && len(r.Meta.Relations) > 0 {
		if err := RunPreloads(ctx, r.Executor, &r.Meta, r.Flavor, &entities, prep.Query.Preloads); err != nil {
			return nil, 0, fmt.Errorf("preload failed: %w", err)
		}
	}

	return entities, totalCount, nil
}

// Get retrieves a record by its ID.
func (r *BaseCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	// Resolve selected columns (Fields selection)
	selectedCols := FieldsToColumns(q.Fields)
	selectCols, _ := r.Meta.FieldIndicesForColumns(selectedCols)

	// Build WHERE clause: pk = ? [AND deleted_at IS NULL]
	whereParts := []string{r.Flavor.WhereEq(r.Meta.PKColumn, 1)}
	if r.Meta.SoftDeleteColumn != "" && !q.IncludeTrashed.Some(true) {
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", r.Meta.SoftDeleteColumn))
	}

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		ColumnsString(selectCols),
		r.Meta.TableName,
		strings.Join(whereParts, " AND "),
	)

	dests := r.Meta.ScanDestForColumns(&entity, selectedCols)
	err := r.Executor.QueryRowContext(ctx, query, id).Scan(dests...)
	if err != nil {
		return entity, err
	}

	// Run preloads if any are requested and the model has relations
	if len(q.Preloads) > 0 && len(r.Meta.Relations) > 0 {
		entities := []T{entity}
		if err := RunPreloads(ctx, r.Executor, &r.Meta, r.Flavor, &entities, q.Preloads); err != nil {
			return entity, fmt.Errorf("preload failed: %w", err)
		}
		entity = entities[0]
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
	_, err := r.Executor.ExecContext(ctx, query, args...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Patch performs a partial update, modifying only the columns specified by fields.
// The entity must have its primary key set. Returns the full entity after the update.
func (r *BaseCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	cols := FieldsToColumns(fields)

	cols, err := r.Meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}

	if !r.Flavor.SupportsRETURNING {
		return r.patchWithoutRETURNING(ctx, entity, cols)
	}

	vals := r.Meta.FieldValuesForColumns(entity, cols)
	pkVal := r.Meta.PKValue(entity)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s RETURNING %s",
		r.Meta.TableName,
		r.Flavor.SetExprs(cols, 1),
		r.Flavor.WhereEq(r.Meta.PKColumn, len(cols)+1),
		ColumnsString(r.Meta.Columns),
	)

	args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
	dests := r.Meta.ScanDest(&entity)
	err = r.Executor.QueryRowContext(ctx, query, args...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// patchWithoutRETURNING is the fallback Patch for databases without RETURNING (MySQL).
func (r *BaseCRUD[T, ID]) patchWithoutRETURNING(ctx context.Context, entity T, cols []string) (T, error) {
	vals := r.Meta.FieldValuesForColumns(entity, cols)
	pkVal := r.Meta.PKValue(entity)

	updateQuery := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.SetExprs(cols, 1),
		r.Flavor.WhereEq(r.Meta.PKColumn, len(cols)+1),
	)

	args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
	_, err := r.Executor.ExecContext(ctx, updateQuery, args...)
	if err != nil {
		return entity, err
	}

	// Re-fetch the full entity
	selectQuery := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		ColumnsString(r.Meta.Columns),
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)

	dests := r.Meta.ScanDest(&entity)
	err = r.Executor.QueryRowContext(ctx, selectQuery, pkVal).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Delete removes a record by its ID.
// If the model has a soft-delete column (tagged with `r3:"soft_delete"`),
// it performs a soft-delete by setting the column to the current timestamp.
// Otherwise, it performs a hard delete.
func (r *BaseCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	var query string
	if r.Meta.SoftDeleteColumn != "" {
		// Soft-delete: UPDATE SET deleted_at = NOW() WHERE pk = ?
		query = fmt.Sprintf(
			"UPDATE %s SET %s = %s WHERE %s",
			r.Meta.TableName,
			r.Meta.SoftDeleteColumn,
			r.Flavor.TimestampFunc,
			r.Flavor.WhereEq(r.Meta.PKColumn, 1),
		)
	} else {
		// Hard delete
		query = fmt.Sprintf(
			"DELETE FROM %s WHERE %s",
			r.Meta.TableName,
			r.Flavor.WhereEq(r.Meta.PKColumn, 1),
		)
	}

	_, err := r.Executor.ExecContext(ctx, query, id)
	return err
}

// Restore clears the soft-delete column, un-deleting a soft-deleted record.
// Returns an error if the model has no soft-delete column.
func (r *BaseCRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	if r.Meta.SoftDeleteColumn == "" {
		return errors.New("r3/engine/sql: model has no soft-delete column")
	}
	query := fmt.Sprintf(
		"UPDATE %s SET %s = NULL WHERE %s",
		r.Meta.TableName,
		r.Meta.SoftDeleteColumn,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)
	_, err := r.Executor.ExecContext(ctx, query, id)
	return err
}

// HardDelete permanently removes a record, ignoring soft-delete.
func (r *BaseCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)
	_, err := r.Executor.ExecContext(ctx, query, id)
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
