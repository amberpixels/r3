package enginesql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

var _ r3.Aggregator = (*BaseCRUD[any, any])(nil)

// PreparedAggregateQuery is the aggregate sibling of PreparedListQuery, consumed
// by any driver (database/sql, GORM, Bun, go-pg). WHERE comes from the embedded
// PreparedListQuery; the aggregate-specific parts are the select list, GROUP BY,
// HAVING, and alias-aware ORDER BY.
type PreparedAggregateQuery struct {
	// PreparedListQuery carries WHERE/joins, pagination, and the merged Query. Its
	// Sorts are already restricted to group fields and aggregate aliases (see
	// r3.Query.AggregateSorts); cursor pagination doesn't apply to grouped rows.
	PreparedListQuery

	// SelectList is the full select list in declaration order: quoted group columns
	// first (each aliased to its raw field name, so result keys are stable across
	// backends), then aggregate expressions with their aliases.
	SelectList []string

	// GroupBy is the group-by expression list (empty = whole-set row): quoted
	// plain columns followed by rendered time-bucket expressions.
	GroupBy []string

	// BucketExprs holds just the rendered time-bucket group expressions, aligned
	// with Query.Buckets. It is a subset of GroupBy, surfaced separately so an ORM
	// driver (which groups by raw field names to let the ORM quote them) can add
	// the bucket expressions without re-deriving them.
	BucketExprs []string

	// Having is the HAVING clause with args (zero when no Having filters). Aliases
	// are already lowered to their aggregate expressions - Postgres disallows
	// select-list aliases in HAVING.
	Having r3sql.SQLClause
}

// PrepareAggregateQuery merges defaults with args, validates against the schema
// (a zero r3.Schema does structural-only validation), and converts to SQL-ready
// aggregate components.
func PrepareAggregateQuery(
	dm *DefaultsManager, schema r3.Schema, flavor Flavor, qarg ...r3.Query,
) (PreparedAggregateQuery, error) {
	return PrepareMergedAggregateQuery(schema, dm.MergeListQuery(qarg...), flavor)
}

// PrepareMergedAggregateQuery builds the SQL aggregate components for an
// already-merged query, exposed separately (like PrepareMergedListQuery) so a
// driver can transform it first (e.g. lower relationship filters). flavor renders
// time-bucket group keys; a flavor without bucket support returns
// [r3.ErrBucketNotSupported] only when the query actually declares buckets.
func PrepareMergedAggregateQuery(schema r3.Schema, q r3.Query, flavor Flavor) (PreparedAggregateQuery, error) {
	var p PreparedAggregateQuery

	if err := schema.ValidateAggregateQuery(q); err != nil {
		return p, err
	}

	// Grouped rows order by group fields/aliases only; default sorts reference
	// entity columns and are dropped. No cursor keyset over grouped rows.
	q = q.Clone()
	q.Sorts = q.AggregateSorts()
	q.Cursor = nil

	base, err := PrepareMergedListQuery(q)
	if err != nil {
		return p, err
	}
	p.PreparedListQuery = base

	groupCols, err := r3sql.GroupByToSQL(q.GroupBy)
	if err != nil {
		return p, err
	}
	p.GroupBy = groupCols

	// Group columns (aliased to raw field names) first, then aggregates.
	// exprByName doubles as the HAVING lowering table.
	exprByName := make(map[string]string, len(q.GroupBy)+len(q.Aggregates))
	p.SelectList = make([]string, 0, len(q.GroupBy)+len(q.Aggregates))
	for i, g := range q.GroupBy {
		name := g.String()
		exprByName[name] = groupCols[i]
		p.SelectList = append(p.SelectList, groupCols[i]+" AS "+r3sql.QuoteIdentifier(name))
	}
	// Time-bucket group keys: a flavor-specific truncation of the (safely quoted)
	// source column, aliased and repeated in GROUP BY. Rendered here (not in the
	// flavor-neutral dialect) because truncation SQL is flavor-specific.
	p.BucketExprs = make([]string, 0, len(q.Buckets))
	for _, b := range q.Buckets {
		col, err := r3sql.SafeColumnExpr(b.Field)
		if err != nil {
			return p, err
		}
		expr, err := flavor.DateTruncExpr(col, b.Unit)
		if err != nil {
			return p, err
		}
		p.GroupBy = append(p.GroupBy, expr)
		p.BucketExprs = append(p.BucketExprs, expr)
		exprByName[b.Alias] = expr
		p.SelectList = append(p.SelectList, expr+" AS "+r3sql.QuoteIdentifier(b.Alias))
	}
	for _, a := range q.Aggregates {
		item, err := r3sql.AggregateToSQL(a)
		if err != nil {
			return p, err
		}
		expr, err := r3sql.AggregateExpr(a)
		if err != nil {
			return p, err
		}
		exprByName[a.Alias] = expr
		p.SelectList = append(p.SelectList, item)
	}

	having, err := r3sql.HavingToSQL(q.Having, exprByName)
	if err != nil {
		return p, err
	}
	p.Having = having

	return p, nil
}

// ScanAggregateRows scans a dynamic-column aggregate result set into
// r3.AggregateRow maps keyed by result column name. []byte values normalize to
// string (MySQL returns text and decimals as []byte); everything else stays
// backend-native, coerced on read by the AggregateRow accessors. Shared by the
// raw-SQL engine and the ORM drivers. The caller owns rows (including Close).
func ScanAggregateRows(rows *sql.Rows) ([]r3.AggregateRow, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []r3.AggregateRow
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(r3.AggregateRow, len(cols))
		for i, c := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[c] = v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Aggregate computes grouped aggregates over the matching records. See
// r3.Aggregator for the semantics.
func (r *BaseCRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	prep, err := PrepareAggregateQuery(&r.DefaultsManager, r.Schema, r.Flavor, qarg...)
	if err != nil {
		return nil, err
	}

	whereParts, whereArgs, joinSQL, argIdx := r.buildFilterSQL(prep.PreparedListQuery)

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = " WHERE " + strings.Join(whereParts, " AND ")
	}

	groupSQL := ""
	if len(prep.GroupBy) > 0 {
		groupSQL = " GROUP BY " + strings.Join(prep.GroupBy, ", ")
	}

	havingSQL := ""
	if prep.Having.Clause != "" {
		converted, _ := r.Flavor.ConvertPlaceholders(prep.Having.Clause, argIdx)
		havingSQL = " HAVING " + converted
		whereArgs = append(whereArgs, prep.Having.Args...)
	}

	var orderSQL string
	if len(prep.Sorts) > 0 {
		sortParts := make([]string, 0, len(prep.Sorts))
		for _, sort := range prep.Sorts {
			sortParts = append(sortParts, sort.String())
		}
		orderSQL = " ORDER BY " + strings.Join(sortParts, ", ")
	}

	var limitSQL string
	if prep.IsPaginated {
		limitSQL = fmt.Sprintf(" LIMIT %d OFFSET %d", prep.Limit, prep.Offset)
	}

	query := r.Flavor.QuoteIdentifiers(fmt.Sprintf(
		"SELECT %s FROM %s%s%s%s%s%s%s",
		strings.Join(prep.SelectList, ", "),
		r.Meta.TableName,
		joinSQL,
		whereSQL,
		groupSQL,
		havingSQL,
		orderSQL,
		limitSQL,
	))

	rows, err := r.Executor.QueryContext(ctx, query, whereArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanAggregateRows(rows)
}
