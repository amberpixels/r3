package r3_test

import (
	"context"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateHelpers(t *testing.T) {
	q := r3.Query{
		GroupBy: r3.GroupBy("location_id", "squad_id"),
		Aggregates: r3.Aggregates{
			r3.AggCount("raids"),
			r3.AggMax("date", "last_date"),
			r3.AggMin("date", "first_date"),
			r3.AggSum("popularity", "total_pop"),
			r3.AggAvg("popularity", "avg_pop"),
			r3.AggCountDistinct("squad_id", "squads"),
		},
	}

	require.Len(t, q.GroupBy, 2)
	assert.Equal(t, "location_id", q.GroupBy[0].String())

	require.Len(t, q.Aggregates, 6)
	assert.Equal(t, r3.AggregateCount, q.Aggregates[0].Func)
	assert.Nil(t, q.Aggregates[0].Field, "AggCount is COUNT(*)")
	assert.Equal(t, "raids", q.Aggregates[0].Alias)
	assert.Equal(t, "date", q.Aggregates[1].Field.String())
	assert.Equal(t, "count_distinct", r3.AggregateCountDistinct.String())
}

func TestAggregateRow_Accessors(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	row := r3.AggregateRow{
		"n_int64":    int64(42),
		"n_int32":    int32(7),
		"n_float":    3.5,
		"n_str":      "1234",                // MySQL decimal arrives as text
		"t_time":     now,                   // native
		"t_sqlite":   "2026-07-08 12:00:00", // SQLite MAX(datetime) loses the type
		"t_rfc3339":  "2026-07-08T12:00:00Z",
		"b_int":      int64(1),
		"s_bytes":    "hello",
		"absent_nil": nil,
	}

	n, ok := row.Int64("n_int64")
	require.True(t, ok)
	assert.Equal(t, int64(42), n)

	n, ok = row.Int64("n_int32")
	require.True(t, ok)
	assert.Equal(t, int64(7), n)

	n, ok = row.Int64("n_str")
	require.True(t, ok)
	assert.Equal(t, int64(1234), n)

	f, ok := row.Float64("n_float")
	require.True(t, ok)
	assert.InDelta(t, 3.5, f, 1e-9)

	f, ok = row.Float64("n_int64")
	require.True(t, ok)
	assert.InDelta(t, 42.0, f, 1e-9)

	tm, ok := row.Time("t_time")
	require.True(t, ok)
	assert.True(t, now.Equal(tm))

	tm, ok = row.Time("t_sqlite")
	require.True(t, ok)
	assert.Equal(t, 2026, tm.Year())

	tm, ok = row.Time("t_rfc3339")
	require.True(t, ok)
	assert.True(t, now.Equal(tm))

	b, ok := row.Bool("b_int")
	require.True(t, ok)
	assert.True(t, b)

	s, ok := row.String("s_bytes")
	require.True(t, ok)
	assert.Equal(t, "hello", s)

	_, ok = row.Int64("absent_nil")
	assert.False(t, ok)
	_, ok = row.Time("missing_key")
	assert.False(t, ok)
}

func TestQuery_CloneAndMergeWith_Aggregation(t *testing.T) {
	base := r3.Query{
		Filters:    r3.Filters{r3.Eq("visible", true)},
		GroupBy:    r3.GroupBy("city_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Gt("n", 1)},
	}

	clone := base.Clone()
	require.Len(t, clone.GroupBy, 1)
	require.Len(t, clone.Aggregates, 1)
	require.Len(t, clone.Having, 1)
	// Deep clone: mutating the clone's specs must not touch the original.
	clone.Aggregates[0].Alias = "changed"
	assert.Equal(t, "n", base.Aggregates[0].Alias)

	// A merge layer without aggregation inherits the base's shape untouched.
	merged := base.MergeWith(r3.Query{Filters: r3.Filters{r3.Eq("country", "DE")}})
	require.Len(t, merged.GroupBy, 1)
	require.Len(t, merged.Aggregates, 1)
	assert.Len(t, merged.Filters, 2, "filters accumulate")

	// A merge layer WITH aggregation overrides the whole shape as a unit.
	merged = base.MergeWith(r3.Query{
		GroupBy:    r3.GroupBy("country"),
		Aggregates: r3.Aggregates{r3.AggSum("popularity", "pop")},
	})
	require.Len(t, merged.GroupBy, 1)
	assert.Equal(t, "country", merged.GroupBy[0].String())
	require.Len(t, merged.Aggregates, 1)
	assert.Equal(t, "pop", merged.Aggregates[0].Alias)
	assert.Empty(t, merged.Having, "having belongs to the overriding shape")
}

func TestQuery_AggregateSorts(t *testing.T) {
	q := r3.Query{
		GroupBy:    r3.GroupBy("city_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts: r3.Sorts{
			r3.NewSortDescSpec(r3.NewFieldSpec("n")),          // alias — kept
			r3.NewSortAscSpec(r3.NewFieldSpec("city_id")),     // group field — kept
			r3.NewSortDescSpec(r3.NewFieldSpec("created_at")), // inherited default — dropped
		},
	}
	sorts := q.AggregateSorts()
	require.Len(t, sorts, 2)
	assert.Equal(t, "n", sorts[0].Column.String())
	assert.Equal(t, "city_id", sorts[1].Column.String())
}

func TestValidateAggregateQuery_Structural(t *testing.T) {
	var zero r3.Schema

	// No aggregates declared.
	err := zero.ValidateAggregateQuery(r3.Query{GroupBy: r3.GroupBy("a")})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Missing alias.
	err = zero.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{{Func: r3.AggregateCount}},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Duplicate alias.
	err = zero.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n"), r3.AggSum("x", "n")},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Alias shadowing a group field.
	err = zero.ValidateAggregateQuery(r3.Query{
		GroupBy:    r3.GroupBy("city_id"),
		Aggregates: r3.Aggregates{r3.AggCount("city_id")},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// SUM without a field.
	err = zero.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{{Func: r3.AggregateSum, Alias: "s"}},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Having referencing an undeclared name.
	err = zero.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Gt("other", 1)},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Relationship filter in Having.
	err = zero.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Has("Squads", r3.Eq("id", 1))},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Valid shape: count + having on the alias, group field in having too.
	err = zero.ValidateAggregateQuery(r3.Query{
		GroupBy:    r3.GroupBy("city_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.And(r3.Gt("n", 1), r3.Ne("city_id", nil))},
	})
	assert.NoError(t, err)
}

func TestValidateAggregateQuery_SchemaCaps(t *testing.T) {
	type Loc struct {
		ID     int64  `r3:"id,pk"`
		Name   string `r3:"name"`
		CityID int64  `r3:"city_id"`
		Bio    string `r3:"bio,no-filter"`
	}
	schema := r3.SchemaOf[Loc]()

	// Unknown group field.
	err := schema.ValidateAggregateQuery(r3.Query{
		GroupBy:    r3.GroupBy("nope"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	require.ErrorIs(t, err, r3.ErrUnknownField)

	// Non-filterable aggregate field.
	err = schema.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{r3.AggMax("bio", "m")},
	})
	require.ErrorIs(t, err, r3.ErrFieldNotFilterable)

	// SUM over a non-numeric attribute.
	err = schema.ValidateAggregateQuery(r3.Query{
		Aggregates: r3.Aggregates{r3.AggSum("name", "s")},
	})
	require.ErrorIs(t, err, r3.ErrInvalidAggregate)

	// Valid: group by city_id, count + max over numeric/known fields.
	err = schema.ValidateAggregateQuery(r3.Query{
		GroupBy:    r3.GroupBy("city_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n"), r3.AggSum("id", "ids")},
	})
	assert.NoError(t, err)
}

// stubQuerier implements r3.Querier without Aggregator.
type stubQuerier struct{}

func (stubQuerier) Get(context.Context, int64, ...r3.Query) (struct{}, error) {
	return struct{}{}, nil
}
func (stubQuerier) List(context.Context, ...r3.Query) ([]struct{}, int64, error) {
	return nil, 0, nil
}
func (stubQuerier) Count(context.Context, ...r3.Query) (int64, error) { return 0, nil }

func TestAggregateOf_NotSupported(t *testing.T) {
	var repo r3.Querier[struct{}, int64] = stubQuerier{}
	_, err := r3.AggregateOf(context.Background(), repo)
	require.ErrorIs(t, err, r3.ErrAggregateNotSupported)
}
