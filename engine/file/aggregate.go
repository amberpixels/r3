package enginefile

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/amberpixels/r3"
)

var _ r3.Aggregator = (*BaseCRUD[any, any])(nil)

// Aggregate computes grouped aggregates in memory. See [r3.Aggregator] for the
// query semantics.
func (r *BaseCRUD[T, ID]) Aggregate(_ context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := r.MergeListQuery(qarg...)
	// The file engine has no schema; structural validation still applies.
	if err := (r3.Schema{}).ValidateAggregateQuery(q); err != nil {
		return nil, err
	}

	entities, err := r.loadAll()
	if err != nil {
		return nil, fmt.Errorf("load entities: %w", err)
	}

	includeTrashed := q.IncludeTrashed.Some() && q.IncludeTrashed.Unwrap()
	entities = r.filterSoftDeleted(entities, includeTrashed)

	if len(q.Filters) > 0 {
		entities, err = filterEntities(entities, q.Filters, &r.Meta)
		if err != nil {
			return nil, fmt.Errorf("apply filters: %w", err)
		}
	}

	rows, err := groupAndFold(entities, q, &r.Meta)
	if err != nil {
		return nil, err
	}

	if len(q.Having) > 0 {
		filtered := make([]r3.AggregateRow, 0, len(rows))
		for _, row := range rows {
			match, err := matchesHavingFilters(row, q.Having)
			if err != nil {
				return nil, fmt.Errorf("apply having: %w", err)
			}
			if match {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}

	sortAggregateRows(rows, q.AggregateSorts())

	if q.Pagination != nil && q.Pagination.IsPaginated() {
		limit, offset := q.Pagination.ToLimitOffset()
		if offset >= len(rows) {
			rows = nil
		} else {
			end := min(offset+limit, len(rows))
			rows = rows[offset:end]
		}
	}

	if rows == nil {
		rows = []r3.AggregateRow{}
	}
	return rows, nil
}

// groupAndFold buckets entities by the group-key tuple (plain group fields plus
// any time-bucket keys, in first-seen order so the result is deterministic even
// unsorted) and folds each declared aggregate over every cell. Empty GroupBy and
// Buckets yields one whole-set cell.
func groupAndFold[T any](entities []T, q r3.Query, meta *StructMeta) ([]r3.AggregateRow, error) {
	groupNames := r3.FieldsToStrings(q.GroupBy)

	type cell struct {
		keys       []any // plain group-field values, aligned with groupNames
		bucketVals []any // truncated bucket values, aligned with q.Buckets
		members    []T
	}
	var order []string
	cells := make(map[string]*cell)

	for _, e := range entities {
		keys := make([]any, len(groupNames))
		keyParts := make([]string, 0, len(groupNames)+len(q.Buckets))
		for i, name := range groupNames {
			v, ok := meta.GetFieldValue(e, name)
			if !ok {
				return nil, fmt.Errorf("unknown field %q in group by", name)
			}
			v = derefValue(v)
			keys[i] = v
			keyParts = append(keyParts, fmt.Sprintf("%v", v))
		}
		bucketVals := make([]any, len(q.Buckets))
		for i, b := range q.Buckets {
			v, ok := meta.GetFieldValue(e, b.Field.String())
			if !ok {
				return nil, fmt.Errorf("unknown field %q in bucket %q", b.Field.String(), b.Alias)
			}
			tv, err := bucketValue(derefValue(v), b.Unit)
			if err != nil {
				return nil, fmt.Errorf("bucket %q: %w", b.Alias, err)
			}
			bucketVals[i] = tv
			keyParts = append(keyParts, fmt.Sprintf("%v", tv))
		}
		key := strings.Join(keyParts, "\x00")
		c, ok := cells[key]
		if !ok {
			c = &cell{keys: keys, bucketVals: bucketVals}
			cells[key] = c
			order = append(order, key)
		}
		c.members = append(c.members, e)
	}

	// A whole-set aggregate (no group keys) over zero rows still yields one row,
	// matching SQL: SELECT COUNT(*) without GROUP BY always returns a row.
	if len(groupNames) == 0 && len(q.Buckets) == 0 && len(cells) == 0 {
		cells[""] = &cell{}
		order = append(order, "")
	}

	rows := make([]r3.AggregateRow, 0, len(order))
	for _, key := range order {
		c := cells[key]
		row := make(r3.AggregateRow, len(groupNames)+len(q.Buckets)+len(q.Aggregates))
		for i, name := range groupNames {
			row[name] = c.keys[i]
		}
		for i, b := range q.Buckets {
			row[b.Alias] = c.bucketVals[i]
		}
		for _, a := range q.Aggregates {
			v, err := foldAggregate(c.members, a, meta)
			if err != nil {
				return nil, err
			}
			row[a.Alias] = v
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// bucketValue truncates a field value to a time-bucket unit. A nil value buckets
// to nil (SQL date_trunc(NULL) = NULL); a non-time value is an error.
func bucketValue(v any, unit r3.BucketUnit) (any, error) {
	if v == nil {
		return nil, nil //nolint:nilnil // SQL NULL: a null timestamp has no bucket
	}
	t, ok := v.(time.Time)
	if !ok {
		return nil, fmt.Errorf("%w: expected a time value, got %T", r3.ErrInvalidBucket, v)
	}
	return r3.TruncateToBucket(t, unit), nil
}

// foldAggregate computes one aggregate over a bucket's members. NULL semantics
// match SQL: aggregates skip null values, COUNT(*) counts rows, and SUM/AVG/
// MIN/MAX over zero non-null values yield nil.
func foldAggregate[T any](members []T, a *r3.AggregateSpec, meta *StructMeta) (any, error) {
	if a.Func == r3.AggregateCount && (a.Field == nil || a.Field.String() == "") {
		return int64(len(members)), nil
	}

	fieldName := a.Field.String()
	values := make([]any, 0, len(members))
	for _, m := range members {
		v, ok := meta.GetFieldValue(m, fieldName)
		if !ok {
			return nil, fmt.Errorf("unknown field %q in aggregate %q", fieldName, a.Alias)
		}
		if v = derefValue(v); v != nil {
			values = append(values, v)
		}
	}

	switch a.Func {
	case r3.AggregateCount:
		return int64(len(values)), nil

	case r3.AggregateCountDistinct:
		distinct := make(map[string]struct{}, len(values))
		for _, v := range values {
			distinct[fmt.Sprintf("%v", v)] = struct{}{}
		}
		return int64(len(distinct)), nil

	case r3.AggregateSum, r3.AggregateAvg:
		if len(values) == 0 {
			return nil, nil //nolint:nilnil // SQL NULL: no non-null values to aggregate
		}
		var sum float64
		allInts := true
		for _, v := range values {
			f, ok := toFloat64(v)
			if !ok {
				return nil, fmt.Errorf("non-numeric value in %s(%q)", a.Func, fieldName)
			}
			if k := reflect.ValueOf(v).Kind(); k == reflect.Float32 || k == reflect.Float64 {
				allInts = false
			}
			sum += f
		}
		if a.Func == r3.AggregateAvg {
			return sum / float64(len(values)), nil
		}
		if allInts {
			return int64(sum), nil
		}
		return sum, nil

	case r3.AggregateMin, r3.AggregateMax:
		if len(values) == 0 {
			return nil, nil //nolint:nilnil // SQL NULL: no non-null values to aggregate
		}
		best := values[0]
		for _, v := range values[1:] {
			cmp := compareOrdered(v, best)
			if (a.Func == r3.AggregateMin && cmp < 0) || (a.Func == r3.AggregateMax && cmp > 0) {
				best = v
			}
		}
		return best, nil

	default:
		return nil, fmt.Errorf("unsupported aggregate function: %v", a.Func)
	}
}

// matchesHavingFilters checks a grouped row against ALL having filters
// (implicit AND), mirroring matchesFilters but reading values from the row.
func matchesHavingFilters(row r3.AggregateRow, filters r3.Filters) (bool, error) {
	for _, f := range filters {
		match, err := matchesHavingFilter(row, f)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

func matchesHavingFilter(row r3.AggregateRow, f *r3.FilterSpec) (bool, error) {
	if f == nil {
		return true, nil
	}
	if f.Relation != "" {
		return false, fmt.Errorf("relationship filter %q has no meaning in having", f.Relation)
	}
	if len(f.And) > 0 {
		for _, child := range f.And {
			match, err := matchesHavingFilter(row, child)
			if err != nil || !match {
				return false, err
			}
		}
		return true, nil
	}
	if len(f.Or) > 0 {
		for _, child := range f.Or {
			match, err := matchesHavingFilter(row, child)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil
			}
		}
		return false, nil
	}
	if f.Field == nil {
		return true, nil
	}
	return evaluateOperator(row[f.Field.String()], f.Operator, f.Value)
}

// sortAggregateRows orders grouped rows by the given sorts (already restricted
// to group fields and aliases via Query.AggregateSorts).
func sortAggregateRows(rows []r3.AggregateRow, sorts r3.Sorts) {
	if len(sorts) == 0 {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		for _, spec := range sorts {
			if spec == nil || spec.Column == nil {
				continue
			}
			aVal := rows[i][spec.Column.String()]
			bVal := rows[j][spec.Column.String()]

			aIsNil, bIsNil := isNullValue(aVal), isNullValue(bVal)
			if aIsNil || bIsNil {
				if aIsNil && bIsNil {
					continue
				}
				return handleNullsPosition(aIsNil, bIsNil, spec.NullsPosition, spec.Direction)
			}

			cmp := compareOrdered(aVal, bVal)
			if cmp == 0 {
				continue
			}
			if spec.Direction == r3.SortDirectionDesc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

// derefValue unwraps pointer values (nil pointer → nil) so grouping keys and
// aggregate inputs compare by value, matching the SQL engines.
func derefValue(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer {
		return v
	}
	if rv.IsNil() {
		return nil
	}
	return rv.Elem().Interface()
}
