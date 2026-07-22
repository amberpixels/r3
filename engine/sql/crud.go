package enginesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/amberpixels/r3"
)

// SQLExecutor is the common interface of *sql.DB and *sql.Tx, letting BaseCRUD
// run transparently in normal and transactional modes.
type SQLExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// BaseCRUD is a generic r3.CRUD[T, ID] over database/sql, using reflection-based
// struct scanning and the r3 SQL dialect. Drivers embed it and may override
// methods (e.g. MySQL overrides Create, lacking RETURNING).
type BaseCRUD[T any, ID comparable] struct {
	// Executor is the *sql.DB or *sql.Tx used for all queries.
	Executor SQLExecutor

	// sqlDB is the original *sql.DB for starting transactions; nil inside a tx.
	sqlDB *sql.DB

	DefaultsManager

	Meta   StructMeta
	Schema r3.Schema // logical schema for read validation and write-shaping
	Flavor Flavor
	Config r3.Config
	Raw    *BaseRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BaseCRUD[any, any]{}

// NewBaseCRUD builds a BaseCRUD for db and flavor. Columns map via `db:"col"`
// tags; the PK is tagged `db:"id,pk"` (defaults to "id"). Options configure
// framework-level behavior (e.g. [r3.WithConfig]).
func NewBaseCRUD[T any, ID comparable](db *sql.DB, flavor Flavor, opts ...r3.Option) *BaseCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	meta := GetStructMeta[T]()
	schema := r3.SchemaOf[T](r3.WithSchemaNaming(resolved.Config.Naming))
	// The raw database/sql engine does not apply value codecs yet; reject a
	// declared codec loudly rather than storing the un-encoded value.
	r3.RequireCodecSupport(schema, "r3/engine/sql")
	return &BaseCRUD[T, ID]{
		Executor:        db,
		sqlDB:           db,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Schema:          schema,
		Flavor:          flavor,
		Config:          resolved.Config,
		Raw:             NewBaseRaw[T, ID](db, meta),
	}
}

// SQLDB returns the underlying *sql.DB for advanced use. Panics inside a
// transaction (use the transaction's CRUD instead).
func (r *BaseCRUD[T, ID]) SQLDB() *sql.DB {
	if r.sqlDB == nil {
		panic("r3/engine/sql: SQLDB() called on a transactional BaseCRUD (use the transaction's CRUD instead)")
	}
	return r.sqlDB
}

// Create inserts a record and returns it with the auto-generated PK populated,
// via RETURNING (Postgres, SQLite 3.35+). MySQL drivers override this.
func (r *BaseCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if !r.Flavor.SupportsRETURNING {
		return r.createWithLastInsertID(ctx, entity)
	}

	cols := r.createColumns(ctx, &entity)
	vals := r.Meta.FieldValuesForColumns(entity, cols)

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

// createColumns returns the columns an INSERT writes and stamps server time on
// managed created_at/updated_at. The caller-writable set is Creatable (or all
// non-PK under a bypass); managed timestamps are appended on top as
// system-written, unless the caller already supplies them (bypass).
func (r *BaseCRUD[T, ID]) createColumns(ctx context.Context, entityPtr *T) []string {
	cols := WriteColumns(ctx, r.Schema, r.Meta.NonPKColumns(), r3.WriteOpCreate)
	return append(cols, r.stampManagedTimestamps(ctx, entityPtr, cols, r3.WriteOpCreate)...)
}

// updateColumns returns the columns an UPDATE writes and stamps server time on
// updated_at. The caller-writable set is Mutable (so read-only columns like
// created_at are never clobbered and a soft-deleted row is not resurrected), or
// all non-PK under a bypass.
func (r *BaseCRUD[T, ID]) updateColumns(ctx context.Context, entityPtr *T) []string {
	cols := WriteColumns(ctx, r.Schema, r.Meta.NonPKColumns(), r3.WriteOpMutate)
	return append(cols, r.stampManagedTimestamps(ctx, entityPtr, cols, r3.WriteOpMutate)...)
}

// stampManagedTimestamps sets server time on the managed timestamp columns this op
// writes and returns them for the caller to add to the write set. No-op under the
// write-guard bypass (worker supplies its own), for a zero schema, and for any
// timestamp the caller already lists.
func (r *BaseCRUD[T, ID]) stampManagedTimestamps(
	ctx context.Context, entityPtr *T, callerCols []string, op r3.WriteOp,
) []string {
	if r.Schema.IsZero() || r3.WriteGuardBypassed(ctx) {
		return nil
	}
	var inject []string
	for _, c := range ManagedTimestampColumns(r.Meta, r.Config.Naming, op) {
		if !slices.Contains(callerCols, c) {
			inject = append(inject, c)
		}
	}
	SetTimeColumns(r.Meta, entityPtr, inject, time.Now())
	return inject
}

// createWithLastInsertID is Create for backends without RETURNING (MySQL).
func (r *BaseCRUD[T, ID]) createWithLastInsertID(ctx context.Context, entity T) (T, error) {
	cols := r.createColumns(ctx, &entity)
	vals := r.Meta.FieldValuesForColumns(entity, cols)

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

// buildFilterSQL converts the prepared query's filters into WHERE parts (with
// flavor placeholders), their args, and a JOIN fragment - the shared core of List
// and Count. The returned argIdx is the next free placeholder index, so List can
// append further clauses (e.g. cursor).
//
// JOINs use JOIN ... ON TRUE: the raw SQL engine lacks the schema metadata to
// infer ON conditions. ORM drivers (GORM, Bun, go-pg) do proper JOINs.
func (r *BaseCRUD[T, ID]) buildFilterSQL(
	prep PreparedListQuery,
) ([]string, []any, string, int) {
	var whereParts []string
	var whereArgs []any
	argIdx := 1

	// Soft-delete: add WHERE deleted_at IS NULL unless IncludeTrashed.
	if r.Meta.SoftDeleteColumn != "" && !prep.Query.IncludeTrashed.Some(true) {
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", r.Meta.SoftDeleteColumn))
	}

	for _, clause := range prep.Clauses {
		converted, nextIdx := r.Flavor.ConvertPlaceholders(clause.Clause, argIdx)
		whereParts = append(whereParts, converted)
		whereArgs = append(whereArgs, clause.Args...)
		argIdx = nextIdx
	}

	var joinSQL string
	if joins := prep.Joins(); len(joins) > 0 {
		var joinParts []string
		for _, join := range joins {
			joinParts = append(joinParts, fmt.Sprintf("JOIN %s ON TRUE", join.String()))
		}
		joinSQL = " " + strings.Join(joinParts, " ")
	}

	return whereParts, whereArgs, joinSQL, argIdx
}

// prepareList merges args with defaults, validates against the schema (an
// unknown/non-filterable/non-sortable field is a typed error before any SQL is
// built; a zero schema validates nothing), then builds the SQL components.
func (r *BaseCRUD[T, ID]) prepareList(qarg ...r3.Query) (PreparedListQuery, error) {
	merged := r.MergeListQuery(qarg...)
	if err := r.Schema.ValidateQuery(merged); err != nil {
		return PreparedListQuery{}, err
	}
	return PrepareMergedListQuery(merged)
}

// Count returns the number of records matching the query's filters.
func (r *BaseCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	prep, err := r.prepareList(qarg...)
	if err != nil {
		return 0, err
	}

	whereParts, whereArgs, joinSQL, _ := r.buildFilterSQL(prep)

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = " WHERE " + strings.Join(whereParts, " AND ")
	}

	countQuery := r.Flavor.QuoteIdentifiers(
		fmt.Sprintf("SELECT COUNT(*) FROM %s%s%s", r.Meta.TableName, joinSQL, whereSQL),
	)

	var total int64
	if err := r.Executor.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// List retrieves records based on the provided query parameters.
func (r *BaseCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := r.prepareList(qarg...)
	if err != nil {
		return nil, 0, err
	}

	whereParts, whereArgs, joinSQL, argIdx := r.buildFilterSQL(prep)

	// Offset pagination counts first; cursor pagination skips the count.
	var totalCount int64
	if prep.IsPaginated {
		countWhereSQL := ""
		if len(whereParts) > 0 {
			countWhereSQL = " WHERE " + strings.Join(whereParts, " AND ")
		}
		countQuery := r.Flavor.QuoteIdentifiers(
			fmt.Sprintf("SELECT COUNT(*) FROM %s%s%s", r.Meta.TableName, joinSQL, countWhereSQL),
		)
		err := r.Executor.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&totalCount)
		if err != nil {
			return nil, 0, err
		}
		if totalCount == 0 {
			return nil, 0, nil
		}
	}

	// Cursor pagination: append the keyset WHERE after the count query.
	if prep.IsCursorPaginated && prep.CursorClause.Clause != "" {
		converted, nextIdx := r.Flavor.ConvertPlaceholders(prep.CursorClause.Clause, argIdx)
		whereParts = append(whereParts, converted)
		whereArgs = append(whereArgs, prep.CursorClause.Args...)
		_ = nextIdx
	}

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = " WHERE " + strings.Join(whereParts, " AND ")
	}

	// Build ORDER BY (reversed for a backward cursor; see OrderBySorts).
	orderSorts, sortErr := prep.OrderBySorts()
	if sortErr != nil {
		return nil, 0, fmt.Errorf("failed to build order by: %w", sortErr)
	}
	var orderSQL string
	if len(orderSorts) > 0 {
		var sortParts []string
		for _, sort := range orderSorts {
			sortParts = append(sortParts, sort.String())
		}
		orderSQL = " ORDER BY " + strings.Join(sortParts, ", ")
	}

	var limitSQL string
	if prep.IsCursorPaginated {
		limitSQL = fmt.Sprintf(" LIMIT %d", prep.CursorLimit)
	} else if prep.IsPaginated {
		limitSQL = fmt.Sprintf(" LIMIT %d OFFSET %d", prep.Limit, prep.Offset)
	}

	selectedCols := FieldsToColumns(prep.Query.Fields)
	selectCols, _ := r.Meta.FieldIndicesForColumns(selectedCols)

	selectQuery := r.Flavor.QuoteIdentifiers(fmt.Sprintf(
		"SELECT %s FROM %s%s%s%s%s",
		ColumnsString(selectCols),
		r.Meta.TableName,
		joinSQL,
		whereSQL,
		orderSQL,
		limitSQL,
	))

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

	// A backward cursor scanned in reversed order; restore the requested order.
	if prep.CursorBackward {
		slices.Reverse(entities)
	}

	if prep.IsCursorPaginated {
		entities, totalCount = r3.FinalizeCountCursor(entities)
	} else {
		entities, totalCount = FinalizeCount(entities, totalCount, prep.IsPaginated)
	}

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
	if err := r.Schema.ValidateQuery(q); err != nil {
		return entity, err
	}

	selectedCols := FieldsToColumns(q.Fields)
	selectCols, _ := r.Meta.FieldIndicesForColumns(selectedCols)

	// WHERE pk = ? [AND deleted_at IS NULL]
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
		if errors.Is(err, sql.ErrNoRows) {
			return entity, r3.ErrNotFound
		}
		return entity, err
	}

	if len(q.Preloads) > 0 && len(r.Meta.Relations) > 0 {
		entities := []T{entity}
		if err := RunPreloads(ctx, r.Executor, &r.Meta, r.Flavor, &entities, q.Preloads); err != nil {
			return entity, fmt.Errorf("preload failed: %w", err)
		}
		entity = entities[0]
	}

	return entity, nil
}

// Update modifies the record identified by its primary key and returns the row
// as persisted.
//
// Only Mutable columns are written, so a full Update cannot clobber created_at or
// resurrect a soft-deleted row; a model with no mutable columns makes it a
// verified no-op. The returned entity carries the stored value of every column
// the write skipped, so a caller-built partial model does not report those as
// zeroed. Bypass the write guard for audited system/worker writes (see
// r3.WithoutWriteGuard).
func (r *BaseCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	cols := r.updateColumns(ctx, &entity)
	if len(cols) == 0 {
		// Nothing to write: read the row back, which both honors the not-found
		// contract and returns persisted state like the writing path does.
		return r.selectByPK(ctx, entity)
	}
	return r.updateReturning(ctx, entity, cols)
}

// Patch updates only the columns named by fields; entity must have its PK set.
// Returns the full entity after the update.
func (r *BaseCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	cols := FieldsToColumns(fields)

	cols, err := r.Meta.ValidatePatchColumns(cols)
	if err != nil {
		return entity, err
	}
	if err := RequireMutableColumns(ctx, r.Schema, cols); err != nil {
		return entity, err
	}
	// A partial update still bumps updated_at (server time), like a full Update.
	cols = append(cols, r.stampManagedTimestamps(ctx, &entity, cols, r3.WriteOpMutate)...)

	return r.updateReturning(ctx, entity, cols)
}

// updateReturning writes cols and reads the row back into entity, the shared
// return path for Update and Patch. RETURNING does both in one statement where
// the flavor supports it (Postgres, SQLite); MySQL follows the UPDATE with a
// SELECT. Scanning lands on the column-backed fields only, so relations the
// caller populated are left alone.
func (r *BaseCRUD[T, ID]) updateReturning(ctx context.Context, entity T, cols []string) (T, error) {
	vals := r.Meta.FieldValuesForColumns(entity, cols)
	pkVal := r.Meta.PKValue(entity)

	if !r.Flavor.SupportsRETURNING {
		query := fmt.Sprintf(
			"UPDATE %s SET %s WHERE %s",
			r.Meta.TableName,
			r.Flavor.SetExprs(cols, 1),
			r.Flavor.WhereEq(r.Meta.PKColumn, len(cols)+1),
		)

		args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
		if _, err := r.Executor.ExecContext(ctx, query, args...); err != nil {
			return entity, err
		}
		// The read-back doubles as the not-found signal, which is the reliable one
		// on MySQL: RowsAffected counts changed (not matched) rows there and can't
		// tell a no-op update from a miss.
		return r.selectByPK(ctx, entity)
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s RETURNING %s",
		r.Meta.TableName,
		r.Flavor.SetExprs(cols, 1),
		r.Flavor.WhereEq(r.Meta.PKColumn, len(cols)+1),
		ColumnsString(r.Meta.Columns),
	)

	args := append(vals, pkVal) //nolint: gocritic // intentional append to new slice
	dests := r.Meta.ScanDest(&entity)
	if err := r.Executor.QueryRowContext(ctx, query, args...).Scan(dests...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No row matched the PK: normalize to the sentinel, like Get.
			return entity, r3.ErrNotFound
		}
		return entity, err
	}
	return entity, nil
}

// selectByPK reads the row identified by entity's PK back into entity's column
// fields, normalizing a missing row to r3.ErrNotFound. It reads the physical row
// regardless of soft-delete state, matching the UPDATE it follows.
func (r *BaseCRUD[T, ID]) selectByPK(ctx context.Context, entity T) (T, error) {
	pkVal := r.Meta.PKValue(entity)

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		ColumnsString(r.Meta.Columns),
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)

	dests := r.Meta.ScanDest(&entity)
	if err := r.Executor.QueryRowContext(ctx, query, pkVal).Scan(dests...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity, r3.ErrNotFound
		}
		return entity, err
	}
	return entity, nil
}

// Delete removes a record by ID. With a soft-delete column (tagged
// `r3:"soft_delete"`) it sets that column to the current timestamp; otherwise it
// hard-deletes.
func (r *BaseCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	if r.Meta.SoftDeleteColumn != "" {
		// Soft-delete: UPDATE SET deleted_at = NOW() WHERE pk = ?
		query := fmt.Sprintf(
			"UPDATE %s SET %s = %s WHERE %s",
			r.Meta.TableName,
			r.Meta.SoftDeleteColumn,
			r.Flavor.TimestampFunc,
			r.Flavor.WhereEq(r.Meta.PKColumn, 1),
		)
		res, err := r.Executor.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}
		return r.requireUpdatedRow(ctx, res, id)
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)
	res, err := r.Executor.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return requireDeletedRow(res)
}

// Restore clears the soft-delete column, un-deleting a record. Errors if the
// model has no soft-delete column.
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
	res, err := r.Executor.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return r.requireUpdatedRow(ctx, res, id)
}

// HardDelete permanently removes a record, ignoring soft-delete.
func (r *BaseCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)
	res, err := r.Executor.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return requireDeletedRow(res)
}

// requireDeletedRow returns r3.ErrNotFound when a DELETE matched no row. A
// DELETE's affected-row count equals its matched-row count on every backend, so
// zero unambiguously means "not found". Skipped when the driver can't report
// RowsAffected.
func requireDeletedRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return nil //nolint:nilerr // driver can't report affected rows; keep prior behavior
	}
	if n == 0 {
		return r3.ErrNotFound
	}
	return nil
}

// requireUpdatedRow returns r3.ErrNotFound when an UPDATE matched no row.
//
// Unlike DELETE, a zero count is ambiguous on MySQL, which reports *changed*
// rather than *matched* rows - a no-op update of an existing row reports zero. So
// on zero we confirm existence by PK before concluding "missing"; the extra
// lookup only runs in that rare case (Postgres/SQLite already count matched rows).
// Skipped when the driver can't report RowsAffected.
func (r *BaseCRUD[T, ID]) requireUpdatedRow(ctx context.Context, res sql.Result, pkVal any) error {
	n, err := res.RowsAffected()
	if err != nil {
		return nil //nolint:nilerr // driver can't report affected rows; keep prior behavior
	}
	if n > 0 {
		return nil
	}

	exists, err := r.existsByPK(ctx, pkVal)
	if err != nil {
		return err
	}
	if !exists {
		return r3.ErrNotFound
	}
	return nil
}

// existsByPK reports whether a row with the given PK exists, regardless of
// soft-delete state (matching the physical row the way UPDATE does).
func (r *BaseCRUD[T, ID]) existsByPK(ctx context.Context, pkVal any) (bool, error) {
	query := fmt.Sprintf(
		"SELECT 1 FROM %s WHERE %s",
		r.Meta.TableName,
		r.Flavor.WhereEq(r.Meta.PKColumn, 1),
	)
	var dummy int
	err := r.Executor.QueryRowContext(ctx, query, pkVal).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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
