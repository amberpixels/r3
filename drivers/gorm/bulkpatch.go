package r3gorm

import (
	"context"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

var _ r3.BulkPatcher[any, any] = &GormCRUD[any, any]{}

// PatchWhere sets fields (to entity's values) on every row matching filters and
// returns the affected-row count. It is the multi-row analogue of Patch - same
// column validation and capability gating (ValidatePatchColumns +
// RequireMutableColumns), same updated_at bump, same write-guard bypass - but rows
// are selected by filters instead of by primary key.
//
// Filters are schema-validated and reuse List's filter-to-SQL translation
// (including relationship "has" filter lowering). Soft-delete exclusion follows
// GORM's default model scoping.
func (r *GormCRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	cols := r3.FieldsToStrings(fields)
	cols, err := r.meta.ValidatePatchColumns(cols)
	if err != nil {
		return 0, err
	}
	if err := enginesql.RequireMutableColumns(ctx, r.schema, cols); err != nil {
		return 0, err
	}
	cols = append(cols, r.stampManaged(ctx, &entity, cols, r3.WriteOpMutate)...)

	if err := r.schema.ValidateQuery(r3.Query{Filters: filters}); err != nil {
		return 0, err
	}

	// Lower relationship filters into key-set In filters before building clauses,
	// exactly as List does.
	if hasRelationFilters(filters) {
		lowered, err := lowerRelationFilters(ctx, r.db, r.meta, filters)
		if err != nil {
			return 0, err
		}
		filters = lowered
	}
	prep, err := enginesql.PrepareMergedListQuery(r3.Query{Filters: filters})
	if err != nil {
		return 0, err
	}

	query := r.db.WithContext(ctx).Model(new(T)).Select(cols)
	for _, join := range prep.Joins() {
		query = query.Joins(join.String())
	}
	for _, clause := range prep.Clauses {
		query = query.Where(clause.Clause, clause.Args...)
	}

	res := query.Updates(entity)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
