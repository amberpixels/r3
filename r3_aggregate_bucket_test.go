package r3_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
)

func TestBucketUnit_StringAndValid(t *testing.T) {
	cases := map[r3.BucketUnit]string{
		r3.BucketHour:  "hour",
		r3.BucketDay:   "day",
		r3.BucketWeek:  "week",
		r3.BucketMonth: "month",
		r3.BucketYear:  "year",
	}
	for u, name := range cases {
		assert.Equal(t, name, u.String())
		assert.True(t, u.Valid())
	}
	assert.Equal(t, "unspecified", r3.BucketUnit(0).String())
	assert.False(t, r3.BucketUnit(0).Valid())
	assert.False(t, r3.BucketUnit(99).Valid())
}

func TestTruncateToBucket(t *testing.T) {
	// 2026-01-07 is a Wednesday, 14:37:09.
	base := time.Date(2026, 1, 7, 14, 37, 9, 500, time.UTC)
	cases := []struct {
		unit r3.BucketUnit
		want time.Time
	}{
		{r3.BucketHour, time.Date(2026, 1, 7, 14, 0, 0, 0, time.UTC)},
		{r3.BucketDay, time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)},
		{r3.BucketWeek, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)}, // Monday
		{r3.BucketMonth, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{r3.BucketYear, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, r3.TruncateToBucket(base, c.unit), "unit %s", c.unit)
	}
}

func TestTruncateToBucket_WeekSundayGoesToPreviousMonday(t *testing.T) {
	// Sunday 2026-01-11 belongs to the ISO week starting Monday 2026-01-05.
	sun := time.Date(2026, 1, 11, 23, 59, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), r3.TruncateToBucket(sun, r3.BucketWeek))
}

func TestTruncateToBucket_PreservesLocation(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	tm := time.Date(2026, 3, 15, 9, 30, 0, 0, loc)
	got := r3.TruncateToBucket(tm, r3.BucketDay)
	assert.Equal(t, loc, got.Location(), "truncation stays in the value's own location")
	assert.Equal(t, time.Date(2026, 3, 15, 0, 0, 0, 0, loc), got)
}

func TestBucket_HelperAndClone(t *testing.T) {
	b := r3.Bucket("created_at", r3.BucketDay, "day")
	assert.Equal(t, "created_at", b.Field.String())
	assert.Equal(t, r3.BucketDay, b.Unit)
	assert.Equal(t, "day", b.Alias)

	clone := b.Clone()
	clone.Alias = "changed"
	assert.Equal(t, "day", b.Alias, "clone is independent")
	assert.NotSame(t, b.Field, clone.Field, "field pointer is deep-cloned")
}

func TestQuery_MergeWithAndCloneCarryBuckets(t *testing.T) {
	req := r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketMonth, "month")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	}
	merged := r3.Query{}.MergeWith(req)
	require.Len(t, merged.Buckets, 1)
	assert.Equal(t, "month", merged.Buckets[0].Alias)

	// The aggregation shape overrides as a unit: a default's buckets are replaced.
	def := r3.Query{Buckets: r3.Buckets{r3.Bucket("created_at", r3.BucketYear, "year")}}
	over := def.MergeWith(req)
	require.Len(t, over.Buckets, 1)
	assert.Equal(t, "month", over.Buckets[0].Alias)

	clone := req.Clone()
	require.Len(t, clone.Buckets, 1)
	assert.NotSame(t, req.Buckets[0], clone.Buckets[0])
}

func TestQuery_AggregateSortsAllowsBucketAlias(t *testing.T) {
	q := r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts: r3.Sorts{
			r3.NewSortAscSpec(r3.NewFieldSpec("day")),         // bucket alias: kept
			r3.NewSortAscSpec(r3.NewFieldSpec("nonexistent")), // dropped
		},
	}
	sorts := q.AggregateSorts()
	require.Len(t, sorts, 1)
	assert.Equal(t, "day", sorts[0].Column.String())
}

func TestValidateAggregateQuery_BucketStructural(t *testing.T) {
	agg := r3.Aggregates{r3.AggCount("n")}

	t.Run("unknown unit", func(t *testing.T) {
		err := r3.Schema{}.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{{Field: r3.NewFieldSpec("created_at"), Unit: r3.BucketUnit(0), Alias: "x"}},
			Aggregates: agg,
		})
		assert.ErrorIs(t, err, r3.ErrInvalidBucket)
	})

	t.Run("missing field", func(t *testing.T) {
		err := r3.Schema{}.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{{Field: r3.NewFieldSpec(""), Unit: r3.BucketDay, Alias: "x"}},
			Aggregates: agg,
		})
		assert.ErrorIs(t, err, r3.ErrInvalidBucket)
	})

	t.Run("duplicate bucket alias", func(t *testing.T) {
		err := r3.Schema{}.ValidateAggregateQuery(r3.Query{
			Buckets: r3.Buckets{
				r3.Bucket("created_at", r3.BucketDay, "d"),
				r3.Bucket("created_at", r3.BucketMonth, "d"), // same alias
			},
			Aggregates: agg,
		})
		assert.ErrorIs(t, err, r3.ErrInvalidBucket)
	})

	t.Run("alias colliding with aggregate is rejected", func(t *testing.T) {
		// A bucket alias colliding with an aggregate alias is still a duplicate;
		// the aggregate loop (which runs after buckets seed the name set) reports
		// it as ErrInvalidAggregate.
		err := r3.Schema{}.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "n")},
			Aggregates: agg, // AggCount alias "n"
		})
		assert.ErrorIs(t, err, r3.ErrInvalidAggregate)
	})

	t.Run("valid passes structural", func(t *testing.T) {
		err := r3.Schema{}.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")},
			Aggregates: agg,
		})
		require.NoError(t, err)
	})
}

// bucketModel exercises schema-aware bucket validation.
type bucketModel struct {
	ID        int       `r3:"id,pk"`
	Name      string    `r3:"name"`
	CreatedAt time.Time `r3:"created_at"`
}

func TestValidateAggregateQuery_BucketSchemaAware(t *testing.T) {
	schema := r3.SchemaOf[bucketModel]()
	agg := r3.Aggregates{r3.AggCount("n")}

	t.Run("time field ok", func(t *testing.T) {
		err := schema.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")},
			Aggregates: agg,
		})
		require.NoError(t, err)
	})

	t.Run("non-time field rejected", func(t *testing.T) {
		err := schema.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{r3.Bucket("name", r3.BucketDay, "day")},
			Aggregates: agg,
		})
		assert.ErrorIs(t, err, r3.ErrInvalidBucket)
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		err := schema.ValidateAggregateQuery(r3.Query{
			Buckets:    r3.Buckets{r3.Bucket("nope", r3.BucketDay, "day")},
			Aggregates: agg,
		})
		assert.ErrorIs(t, err, r3.ErrUnknownField)
	})
}
