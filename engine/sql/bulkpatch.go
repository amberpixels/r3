package enginesql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

var _ r3.BulkPatcher[any, any] = &BaseCRUD[any, any]{}

// PatchWhere sets the given fields (to entity's values) on every row matching
// filters and returns the affected-row count. It is the multi-row analogue of
// Patch: the column set is validated and capability-gated identically
// (ValidatePatchColumns + RequireMutableColumns), managed updated_at is bumped,
// and the write guard bypass is honored — but rows are selected by filters
// instead of by primary key.
//
// Filters are validated against the schema (an unknown or non-filterable field
// is a typed error, as in List). Soft-deleted rows are excluded by default, so a
// bulk update never touches trashed rows. Relationship ("has") filters are not
// supported here and return an error.
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

// buildBulkWhere translates filters into a WHERE clause whose placeholders start
// at startIdx (so they follow the SET clause's parameters), plus the soft-delete
// guard. It rejects relationship filters, which would require a JOIN that a bare
// UPDATE cannot express portably.
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
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", r.Meta.SoftDeleteColumn))
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
