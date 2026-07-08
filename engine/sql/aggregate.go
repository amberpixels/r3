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

// PreparedAggregateQuery holds pre-computed SQL components for an Aggregate
// call, the aggregate sibling of PreparedListQuery. Any driver (database/sql,
// GORM, Bun, go-pg) consumes the same pieces: WHERE comes from the embedded
// PreparedListQuery, the aggregate-specific parts are the select list,
// GROUP BY columns, HAVING clause, and alias-aware ORDER BY.
type PreparedAggregateQuery struct {
	// PreparedListQuery carries the WHERE clauses/joins, pagination, and the
	// merged Query. Its Sorts are already restricted to group fields and
	// aggregate aliases (see r3.Query.AggregateSorts); cursor pagination does
	// not apply to grouped rows.
	PreparedListQuery

	// SelectList is the full select list in declaration order: quoted group
	// columns first (each aliased to its raw field name, so result keys are
	// stable across backends), then aggregate expressions with their aliases.
	SelectList []string

	// GroupBy is the quoted group-by column list (empty = whole-set row).
	GroupBy []string

	// Having is the HAVING clause with args (zero when no Having filters).
	// Aliases are already lowered to their aggregate expressions (portable —
	// PostgreSQL does not allow select-list aliases in HAVING).
	Having r3sql.SQLClause
}

// PrepareAggregateQuery merges defaults with user query args, validates the
// merged query against the schema (pass a zero r3.Schema for structural-only
// validation), and converts it into SQL-ready aggregate components.
func PrepareAggregateQuery(
	dm *DefaultsManager, schema r3.Schema, qarg ...r3.Query,
) (PreparedAggregateQuery, error) {
	return PrepareMergedAggregateQuery(schema, dm.MergeListQuery(qarg...))
}

// PrepareMergedAggregateQuery builds the SQL aggregate components for an
// already-merged query. Like PrepareMergedListQuery, it is exposed separately
// so a driver can transform the merged query (e.g. lower relationship filters)
// before the clauses are built.
func PrepareMergedAggregateQuery(schema r3.Schema, q r3.Query) (PreparedAggregateQuery, error) {
	var p PreparedAggregateQuery

	if err := schema.ValidateAggregateQuery(q); err != nil {
		return p, err
	}

	// Grouped rows are ordered by group fields / aliases only; sorts inherited
	// from repo defaults reference entity columns and are dropped. Cursor
	// pagination has no keyset over grouped rows.
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

	// Select list: group columns (aliased to their raw field names) first,
	// then the aggregates. exprByName doubles as the HAVING lowering table.
	exprByName := make(map[string]string, len(q.GroupBy)+len(q.Aggregates))
	p.SelectList = make([]string, 0, len(q.GroupBy)+len(q.Aggregates))
	for i, g := range q.GroupBy {
		name := g.String()
		exprByName[name] = groupCols[i]
		p.SelectList = append(p.SelectList, groupCols[i]+" AS "+r3sql.QuoteIdentifier(name))
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

// ScanAggregateRows scans a dynamic-column result set (as produced by an
// aggregate SELECT) into r3.AggregateRow maps keyed by the result column
// names. []byte values are normalized to string (MySQL returns text and
// decimals as []byte); everything else stays backend-native — the
// AggregateRow accessors coerce on read. Shared by the raw-SQL engine and the
// ORM drivers. The caller owns rows (including Close).
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

// Aggregate computes grouped aggregates over the records matching the query.
// See r3.Aggregator for the query semantics.
func (r *BaseCRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	prep, err := PrepareAggregateQuery(&r.DefaultsManager, r.Schema, qarg...)
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
