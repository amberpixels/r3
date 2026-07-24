package enginesql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

// sqliteWeekWant is the SQLite ISO-Monday week truncation, kept on its own lines
// so no single source line runs long.
const sqliteWeekWant = `date("created_at", '-' || ` +
	`((strftime('%w', "created_at") + 6) % 7) || ' days')`

// mysqlWeekWant is the MySQL ISO-Monday week truncation (WEEKDAY is 0=Monday).
const mysqlWeekWant = `DATE(DATE_SUB("created_at", ` +
	`INTERVAL WEEKDAY("created_at") DAY))`

func TestFlavorDateTruncExpr_Goldens(t *testing.T) {
	const col = `"created_at"`
	cases := []struct {
		flavor enginesql.Flavor
		name   string
		unit   r3.BucketUnit
		want   string
	}{
		{enginesql.FlavorPostgres, "pg", r3.BucketDay, `date_trunc('day', "created_at")`},
		{enginesql.FlavorPostgres, "pg", r3.BucketWeek, `date_trunc('week', "created_at")`},
		{enginesql.FlavorPostgres, "pg", r3.BucketMonth, `date_trunc('month', "created_at")`},
		{enginesql.FlavorSQLite, "sqlite", r3.BucketDay, `date("created_at")`},
		{enginesql.FlavorSQLite, "sqlite", r3.BucketMonth, `strftime('%Y-%m-01', "created_at")`},
		{enginesql.FlavorSQLite, "sqlite", r3.BucketWeek, sqliteWeekWant},
		{enginesql.FlavorMySQL, "mysql", r3.BucketDay, `DATE("created_at")`},
		{enginesql.FlavorMySQL, "mysql", r3.BucketMonth, `DATE_FORMAT("created_at", '%Y-%m-01')`},
		{enginesql.FlavorMySQL, "mysql", r3.BucketWeek, mysqlWeekWant},
	}
	for _, c := range cases {
		t.Run(c.name+"/"+c.unit.String(), func(t *testing.T) {
			got, err := c.flavor.DateTruncExpr(col, c.unit)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestFlavorDateTruncExpr_UnknownUnit(t *testing.T) {
	_, err := enginesql.FlavorPostgres.DateTruncExpr(`"c"`, r3.BucketUnit(0))
	assert.ErrorIs(t, err, r3.ErrInvalidBucket)
}

// A zero Flavor has no bucket hook: a bucket query must fail loudly, never emit
// un-bucketed SQL. Plain aggregates on a zero flavor still work.
func TestPrepareAggregate_ZeroFlavorBucketDegradesLoudly(t *testing.T) {
	q := r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	}
	_, err := enginesql.PrepareMergedAggregateQuery(r3.Schema{}, q, enginesql.Flavor{})
	assert.ErrorIs(t, err, r3.ErrBucketNotSupported)
}

func TestPrepareAggregate_BucketRendersIntoSelectAndGroupBy(t *testing.T) {
	q := r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	}
	prep, err := enginesql.PrepareMergedAggregateQuery(r3.Schema{}, q, enginesql.FlavorSQLite)
	require.NoError(t, err)

	require.Len(t, prep.BucketExprs, 1)
	assert.Equal(t, `date("created_at")`, prep.BucketExprs[0])
	assert.Contains(t, prep.GroupBy, `date("created_at")`)
	assert.Contains(t, prep.SelectList, `date("created_at") AS "day"`)
}
