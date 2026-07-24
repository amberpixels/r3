package enginesql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

var _ r3.BulkPatcher[any, any] = &BaseCRUD[any, any]{}

// PatchWhere sets fields (from entity's values) on every row matching filters and
// returns the affected-row count - the multi-row Patch: same column validation
// and capability gating (ValidatePatchColumns + RequireMutableColumns), managed
// updated_at bump, and write-guard bypass, but rows are selected by filters.
//
// Filters are schema-validated (an unknown or non-filterable field is a typed
// error, as in List). Soft-deleted rows are excluded by default. Relationship
// ("has") filters are not supported and return an error.
func (r *BaseCRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	cols := FieldsToColumns(fields)
	cols, err := r.Meta.ValidatePatchColumns(cols)
	if err != nil {
		return 0, err
	}
	if err := RequireMutableColumns(ctx, r.Schema, cols); err != nil {
		return 0, err
	}
	cols = append(cols, r.stampManagedTimestamps(ctx, &entity, cols, r3.WriteOpMutate)...)

	if err := r.Schema.ValidateQuery(r3.Query{Filters: filters}); err != nil {
		return 0, err
	}

	whereSQL, whereArgs, err := r.buildBulkWhere(filters, len(cols)+1)
	if err != nil {
		return 0, err
	}

	vals := r.Meta.FieldValuesForColumns(entity, cols)
	query := r.Flavor.QuoteIdentifiers(fmt.Sprintf(
		"UPDATE %s SET %s%s",
		r.Meta.TableName,
		r.Flavor.SetExprs(cols, 1),
		whereSQL,
	))

	args := append(vals, whereArgs...) //nolint:gocritic // intentional append to new slice
	res, err := r.Executor.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil //nolint:nilerr // driver can't report affected rows
	}
	return n, nil
}

// buildBulkWhere builds a WHERE clause (plus the soft-delete guard) whose
// placeholders start at startIdx, following the SET parameters. It rejects
// relationship filters, which need a JOIN a bare UPDATE can't express portably.
func (r *BaseCRUD[T, ID]) buildBulkWhere(filters r3.Filters, startIdx int) (string, []any, error) {
	prep, err := PrepareMergedListQuery(r3.Query{Filters: filters})
	if err != nil {
		return "", nil, err
	}
	if joins := prep.Joins(); len(joins) > 0 {
		return "", nil, errors.New("r3/engine/sql: PatchWhere does not support relationship filters")
	}

	var whereParts []string
	var whereArgs []any
	if r.Meta.SoftDeleteColumn != "" {
		whereParts = append(whereParts, r.Meta.SoftDeleteColumn+" IS NULL")
	}

	argIdx := startIdx
	for _, clause := range prep.Clauses {
		converted, nextIdx := r.Flavor.ConvertPlaceholders(clause.Clause, argIdx)
		whereParts = append(whereParts, converted)
		whereArgs = append(whereArgs, clause.Args...)
		argIdx = nextIdx
	}

	if len(whereParts) == 0 {
		return "", nil, nil
	}
	return " WHERE " + strings.Join(whereParts, " AND "), whereArgs, nil
}
