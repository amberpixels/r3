package r3

import (
	"context"
	"math"
	"strconv"
	"time"
)

// strUnspecified is the shared String() fallback for unset enum-ish specs
// (filter operators, sort directions, aggregate functions).
const strUnspecified = "unspecified"

// AggregateFunc identifies an aggregate function applied over a group of rows.
type AggregateFunc int8

const (
	// AggregateCount counts rows in the group (COUNT(*) when Field is nil,
	// COUNT(field) otherwise — NULLs excluded).
	AggregateCount AggregateFunc = iota + 1
	// AggregateCountDistinct counts distinct non-NULL values of Field.
	AggregateCountDistinct
	// AggregateSum sums Field over the group.
	AggregateSum
	// AggregateAvg averages Field over the group.
	AggregateAvg
	// AggregateMin takes the minimum of Field over the group.
	AggregateMin
	// AggregateMax takes the maximum of Field over the group.
	AggregateMax
)

// String returns the canonical lowercase name of the aggregate function.
func (f AggregateFunc) String() string {
	switch f {
	case AggregateCount:
		return "count"
	case AggregateCountDistinct:
		return "count_distinct"
	case AggregateSum:
		return "sum"
	case AggregateAvg:
		return "avg"
	case AggregateMin:
		return "min"
	case AggregateMax:
		return "max"
	default:
		return strUnspecified
	}
}

// AggregateSpec declares one aggregated value to compute per group: the
// function, the field it aggregates (nil only for AggregateCount = COUNT(*)),
// and the alias under which the value appears in each [AggregateRow]. The
// alias is also the name Sorts and Having refer to.
type AggregateSpec struct {
	Func  AggregateFunc
	Field *FieldSpec
	Alias string
}

// Clone returns a deep copy of the spec.
func (a *AggregateSpec) Clone() *AggregateSpec {
	if a == nil {
		return nil
	}
	clone := *a
	if a.Field != nil {
		clone.Field = a.Field.Clone()
	}
	return &clone
}

// Aggregates is a slice of *AggregateSpec.
type Aggregates []*AggregateSpec

// Clone returns a safe full-clone of the aggregates list.
func (as Aggregates) Clone() Aggregates {
	if as == nil {
		return nil
	}
	cloned := make(Aggregates, len(as))
	for i, a := range as {
		cloned[i] = a.Clone()
	}
	return cloned
}

//
// Ergonomic short-form helpers, mirroring the filter sugar (Eq, Gt, In, ...).
//

// AggCount counts rows per group: COUNT(*) AS alias.
func AggCount(alias string) *AggregateSpec {
	return &AggregateSpec{Func: AggregateCount, Alias: alias}
}

// AggCountDistinct counts distinct values: COUNT(DISTINCT field) AS alias.
func AggCountDistinct(field, alias string) *AggregateSpec {
	return &AggregateSpec{Func: AggregateCountDistinct, Field: NewFieldSpec(field), Alias: alias}
}

// AggSum builds SUM(field) AS alias.
func AggSum(field, alias string) *AggregateSpec {
	return &AggregateSpec{Func: AggregateSum, Field: NewFieldSpec(field), Alias: alias}
}

// AggAvg builds AVG(field) AS alias.
func AggAvg(field, alias string) *AggregateSpec {
	return &AggregateSpec{Func: AggregateAvg, Field: NewFieldSpec(field), Alias: alias}
}

// AggMin builds MIN(field) AS alias.
func AggMin(field, alias string) *AggregateSpec {
	return &AggregateSpec{Func: AggregateMin, Field: NewFieldSpec(field), Alias: alias}
}

// AggMax builds MAX(field) AS alias.
func AggMax(field, alias string) *AggregateSpec {
	return &AggregateSpec{Func: AggregateMax, Field: NewFieldSpec(field), Alias: alias}
}

// GroupBy builds the Fields list for Query.GroupBy from plain field names:
//
//	r3.Query{GroupBy: r3.GroupBy("location_id", "squad_id"), ...}
func GroupBy(fields ...string) Fields {
	out := make(Fields, 0, len(fields))
	for _, f := range fields {
		out = append(out, NewFieldSpec(f))
	}
	return out
}

// AggregateRow is one result row of an Aggregate call: group-field values plus
// one entry per declared aggregate alias.
//
// Raw values are backend-native (e.g. SQLite returns MAX over a timestamp
// column as TEXT, MySQL returns SUM as a decimal string), so prefer the typed
// accessors — they coerce the common cross-backend representations.
type AggregateRow map[string]any

// Value returns the raw backend value for key (nil if absent).
func (r AggregateRow) Value(key string) any { return r[key] }

// String returns the value as a string. []byte is converted; other types
// report ok=false.
func (r AggregateRow) String(key string) (string, bool) {
	switch v := r[key].(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	default:
		return "", false
	}
}

// Int64 returns the value as int64, coercing integer widths, integral floats,
// and numeric strings (MySQL decimals arrive as []byte).
func (r AggregateRow) Int64(key string) (int64, bool) {
	switch v := r[key].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int16:
		return int64(v), true
	case int8:
		return int64(v), true
	case uint64:
		if v > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case uint32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		return n, err == nil
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

// Float64 returns the value as float64, coercing integer widths and numeric
// strings (MySQL decimals arrive as []byte).
func (r AggregateRow) Float64(key string) (float64, bool) {
	switch v := r[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	case []byte:
		f, err := strconv.ParseFloat(string(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// Bool returns the value as bool, coercing the 0/1 integers SQL backends
// return for boolean columns.
func (r AggregateRow) Bool(key string) (bool, bool) {
	switch v := r[key].(type) {
	case bool:
		return v, true
	case int64:
		return v != 0, true
	case int:
		return v != 0, true
	default:
		return false, false
	}
}

// timeLayouts are the textual timestamp formats aggregate values arrive in
// when the backend loses the column's time type (SQLite MIN/MAX over a
// datetime column returns TEXT).
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// Time returns the value as time.Time, parsing the textual forms backends fall
// back to when an aggregate loses the column's declared type.
func (r AggregateRow) Time(key string) (time.Time, bool) {
	switch v := r[key].(type) {
	case time.Time:
		return v, true
	case *time.Time:
		if v == nil {
			return time.Time{}, false
		}
		return *v, true
	case string:
		return parseTime(v)
	case []byte:
		return parseTime(string(v))
	default:
		return time.Time{}, false
	}
}

func parseTime(s string) (time.Time, bool) {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Aggregator is the optional aggregation capability of a repository: grouped
// COUNT/SUM/AVG/MIN/MAX over the records matching a query, without
// materializing entities.
//
// Of the merged Query, Aggregate honors Filters, IncludeTrashed, GroupBy,
// Aggregates, Having, Sorts (over group fields and aggregate aliases only —
// other sorts, e.g. inherited defaults, are ignored), and Pagination (limits
// grouped rows). Fields, Preloads, and Cursor are ignored. An empty GroupBy
// with non-empty Aggregates yields a single whole-set row; empty Aggregates is
// ErrInvalidAggregate.
//
// All r3 engines implement Aggregator, and every feature decorator forwards it
// (applying its concern — permissions gates and scopes, metrics instruments),
// so the capability survives decoration. Reach it via [AggregateOf] rather
// than asserting inner repos yourself: unwrapping a decorator chain would
// silently bypass permission scoping.
type Aggregator interface {
	// Aggregate computes the declared aggregates per group and returns one
	// row per group.
	Aggregate(ctx context.Context, qarg ...Query) ([]AggregateRow, error)
}

// AggregateSorts returns the query's Sorts restricted to what a grouped result
// can be ordered by: group fields and aggregate aliases. Anything else —
// typically a default sort inherited from the repo's DefaultListQuery, which
// references entity columns that no longer exist in grouped rows — is dropped.
// Engines call this instead of using Query.Sorts directly.
func (q Query) AggregateSorts() Sorts {
	if len(q.Sorts) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(q.GroupBy)+len(q.Aggregates))
	for _, g := range q.GroupBy {
		allowed[g.String()] = struct{}{}
	}
	for _, a := range q.Aggregates {
		if a != nil {
			allowed[a.Alias] = struct{}{}
		}
	}
	out := make(Sorts, 0, len(q.Sorts))
	for _, s := range q.Sorts {
		if s == nil || s.Column == nil {
			continue
		}
		if _, ok := allowed[s.Column.String()]; ok {
			out = append(out, s)
		}
	}
	return out
}

// AggregateOf runs an aggregate query against repo if it (including its
// decorators, which all forward the capability) implements [Aggregator], and
// returns ErrAggregateNotSupported otherwise. It deliberately asserts only the
// outermost value — never walking the decorator chain — so feature concerns
// (permission scoping in particular) always apply.
func AggregateOf[T any, ID comparable](
	ctx context.Context, repo Querier[T, ID], qarg ...Query,
) ([]AggregateRow, error) {
	agg, ok := repo.(Aggregator)
	if !ok {
		return nil, ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}
