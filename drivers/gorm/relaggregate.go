package r3gorm

import (
	"context"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

var _ r3.RelationAggregator = &GormCRUD[any, any]{}

// AggregateThroughRelation aggregates the entity's related rows through a
// declared relation, grouped and folded over the related base table (the child
// table for has-many, the join table for many-to-many). See
// [r3.RelationAggregator] for the full query semantics.
func (r *GormCRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	rel, ok := findRelation(r.meta, relation)
	if !ok {
		return nil, fmt.Errorf("relation aggregate: unknown relation %q on %s", relation, r.meta.TableName)
	}
	plan, err := buildRelationAggregatePlan(rel)
	if err != nil {
		return nil, err
	}

	// Merge only the caller queries — not the entity's default list query, whose
	// default sorts and pagination reference the owner's own columns, which do
	// not exist in the related base table.
	var q r3.Query
	for _, a := range qarg {
		q = q.MergeWith(a)
	}
	// Structural (shape-only) validation: at least one aggregate, unique/valid
	// aliases, valid group and having identifiers. The related base table carries
	// no r3 schema (it may be a bare join table or an unimported target), so the
	// DB reports any unknown column.
	if err := (r3.Schema{}).ValidateAggregateQuery(q); err != nil {
		return nil, err
	}

	selectList, exprByName, err := buildRelationAggregateSelect(q)
	if err != nil {
		return nil, err
	}
	having, err := r3sql.HavingToSQL(q.Having, exprByName)
	if err != nil {
		return nil, err
	}

	query := r.db.WithContext(ctx).Table(plan.baseTable)
	if plan.joinClause != "" {
		query = query.Joins(plan.joinClause)
	}
	if plan.softDeleteCond != "" {
		query = query.Where(plan.softDeleteCond)
	}
	// Owner filters restrict which owners' related rows are aggregated — this is
	// where a permissions Scoper's owner filters land. Applied as a subquery on
	// the owner table rather than a materialized key set.
	if len(q.Filters) > 0 {
		clause, args, err := r.ownerKeyRestriction(ctx, plan.ownerKeyColumn, q.Filters)
		if err != nil {
			return nil, err
		}
		query = query.Where(clause, args...)
	}

	query = query.Select(strings.Join(selectList, ", "))
	// Group by raw field names one at a time: gorm quotes each per its dialect
	// (mirrors the single-table Aggregate path).
	for _, g := range q.GroupBy {
		query = query.Group(g.String())
	}
	if having.Clause != "" {
		query = query.Having(having.Clause, having.Args...)
	}
	for _, s := range q.AggregateSorts() {
		query = query.Order(s.String())
	}
	if q.Pagination != nil && q.Pagination.IsPaginated() {
		limit, offset := q.Pagination.ToLimitOffset()
		query = query.Limit(limit).Offset(offset)
	}

	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return enginesql.ScanAggregateRows(rows)
}

// relationAggregatePlan captures how to aggregate a relation's related rows:
// which table to scan and group, the owner-side key column in it, and the
// optional target join used to exclude soft-deleted related rows.
type relationAggregatePlan struct {
	baseTable      string // table scanned & grouped (child table, or join table for m2m)
	ownerKeyColumn string // column in baseTable that identifies the owner
	joinClause     string // "JOIN target ON ..." for m2m soft-delete exclusion, else ""
	softDeleteCond string // "<col> IS NULL" predicate excluding soft-deleted related rows, else ""
}

func buildRelationAggregatePlan(rel enginesql.RelationMeta) (relationAggregatePlan, error) {
	switch rel.Kind {
	case enginesql.RelHasMany:
		// Related rows live in the child table, keyed to the owner by FKColumn.
		// base table == target table, so soft-delete is a plain predicate.
		plan := relationAggregatePlan{
			baseTable:      rel.TargetMeta.TableName,
			ownerKeyColumn: rel.FKColumn,
		}
		if sd := rel.TargetMeta.SoftDeleteColumn; sd != "" {
			plan.softDeleteCond = r3sql.QuoteIdentifier(sd) + " IS NULL"
		}
		return plan, nil

	case enginesql.RelManyToMany:
		if rel.JoinTable == "" || rel.RefColumn == "" {
			return relationAggregatePlan{}, fmt.Errorf(
				"relation aggregate: many-to-many relation %q is missing join table or ref column", rel.FieldName)
		}
		// Related rows are counted from the join table; a target join is added
		// only to exclude soft-deleted related rows.
		plan := relationAggregatePlan{
			baseTable:      rel.JoinTable,
			ownerKeyColumn: rel.FKColumn,
		}
		if sd := rel.TargetMeta.SoftDeleteColumn; sd != "" {
			target := r3sql.QuoteIdentifier(rel.TargetMeta.TableName)
			join := r3sql.QuoteIdentifier(rel.JoinTable)
			plan.joinClause = fmt.Sprintf("JOIN %s ON %s.%s = %s.%s",
				target, target, r3sql.QuoteIdentifier(rel.TargetMeta.PKColumn),
				join, r3sql.QuoteIdentifier(rel.RefColumn))
			plan.softDeleteCond = target + "." + r3sql.QuoteIdentifier(sd) + " IS NULL"
		}
		return plan, nil

	default:
		return relationAggregatePlan{}, fmt.Errorf(
			"relation aggregate: relation %q is not aggregatable "+
				"(only has-many and many-to-many are supported)", rel.FieldName)
	}
}

// buildRelationAggregateSelect builds the SELECT list (group columns aliased to
// their own name, plus each aggregate) and the exprByName map that HAVING uses
// to resolve aliases and group names to bare expressions. Columns resolve
// against the related base table. Mirrors the single-table aggregate builder in
// engine/sql.
func buildRelationAggregateSelect(q r3.Query) ([]string, map[string]string, error) {
	groupCols, err := r3sql.GroupByToSQL(q.GroupBy)
	if err != nil {
		return nil, nil, err
	}
	selectList := make([]string, 0, len(q.GroupBy)+len(q.Aggregates))
	exprByName := make(map[string]string, len(q.GroupBy)+len(q.Aggregates))
	for i, g := range q.GroupBy {
		name := g.String()
		selectList = append(selectList, groupCols[i]+" AS "+r3sql.QuoteIdentifier(name))
		exprByName[name] = groupCols[i]
	}
	for _, a := range q.Aggregates {
		item, err := r3sql.AggregateToSQL(a)
		if err != nil {
			return nil, nil, err
		}
		selectList = append(selectList, item)
		expr, err := r3sql.AggregateExpr(a)
		if err != nil {
			return nil, nil, err
		}
		exprByName[a.Alias] = expr
	}
	return selectList, exprByName, nil
}

// ownerKeyRestriction builds `<ownerKeyColumn> IN (SELECT <ownerPK> FROM
// <ownerTable> WHERE <ownerFilters> [AND <ownerSoftDelete> IS NULL])`, so owner
// filters (including a permissions Scoper's) restrict which owners' related rows
// are aggregated. Owner filters may reference relations; they are lowered first.
func (r *GormCRUD[T, ID]) ownerKeyRestriction(
	ctx context.Context, ownerKeyColumn string, ownerFilters r3.Filters,
) (string, []any, error) {
	if hasRelationFilters(ownerFilters) {
		lowered, err := lowerRelationFilters(ctx, r.db, r.meta, ownerFilters)
		if err != nil {
			return "", nil, err
		}
		ownerFilters = lowered
	}
	clauses, err := r3sql.FiltersToSQL(ownerFilters)
	if err != nil {
		return "", nil, fmt.Errorf("relation aggregate: translate owner filters: %w", err)
	}

	sub := r.db.WithContext(ctx).Table(r.meta.TableName).Select(r.meta.PKColumn)
	if r.meta.SoftDeleteColumn != "" {
		sub = sub.Where(r3sql.QuoteIdentifier(r.meta.SoftDeleteColumn) + " IS NULL")
	}
	for _, c := range clauses {
		sub = sub.Where(c.Clause, c.Args...)
	}
	return r3sql.QuoteIdentifier(ownerKeyColumn) + " IN (?)", []any{sub}, nil
}
