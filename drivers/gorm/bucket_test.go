package r3gorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// TestGormAggregate_BucketByDay pushes a "count per day" grouping down to SQLite
// via the date() truncation hook, the flagship use case from issue #3.
func TestGormAggregate_BucketByDay(t *testing.T) {
	db := setupAggDB(t)
	repo := r3gorm.NewGormCRUD[aggRaid, int64](db)
	ctx := context.Background()

	utc := func(y int, mo time.Month, d, h int) time.Time {
		return time.Date(y, mo, d, h, 0, 0, 0, time.UTC)
	}
	for _, raid := range []aggRaid{
		{Date: utc(2026, 1, 5, 9)},  // Mon
		{Date: utc(2026, 1, 5, 20)}, // Mon, same day
		{Date: utc(2026, 1, 6, 10)}, // Tue, same ISO week
		{Date: utc(2026, 1, 12, 8)}, // next Mon, next week
		{Date: utc(2026, 2, 3, 8)},  // February
	} {
		_, err := repo.Create(ctx, raid)
		require.NoError(t, err)
	}

	rows, err := repo.Aggregate(ctx, r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("date", r3.BucketDay, "day")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("day"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 4) // 01-05, 01-06, 01-12, 02-03

	day0, ok := rows[0].Time("day")
	require.True(t, ok, "Time() must parse sqlite date(), got %#v", rows[0].Value("day"))
	assert.Equal(t, "2026-01-05", day0.Format(time.DateOnly))
	n0, _ := rows[0].Int64("n")
	assert.Equal(t, int64(2), n0)
}

// TestGormAggregate_BucketByWeek verifies the ISO-Monday week start survives the
// SQLite %w Monday-shift.
func TestGormAggregate_BucketByWeek(t *testing.T) {
	db := setupAggDB(t)
	repo := r3gorm.NewGormCRUD[aggRaid, int64](db)
	ctx := context.Background()

	utc := func(mo time.Month, d int) time.Time {
		return time.Date(2026, mo, d, 12, 0, 0, 0, time.UTC)
	}
	for _, raid := range []aggRaid{
		{Date: utc(1, 5)},  // Mon
		{Date: utc(1, 6)},  // Tue, same week
		{Date: utc(1, 11)}, // Sun, still the 01-05 ISO week
		{Date: utc(1, 12)}, // next Mon
	} {
		_, err := repo.Create(ctx, raid)
		require.NoError(t, err)
	}

	rows, err := repo.Aggregate(ctx, r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("date", r3.BucketWeek, "week")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("week"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	week0, ok := rows[0].Time("week")
	require.True(t, ok)
	assert.Equal(t, "2026-01-05", week0.Format(time.DateOnly))
	n0, _ := rows[0].Int64("n")
	assert.Equal(t, int64(3), n0) // Mon+Tue+Sun collapse into the 01-05 week
}

// TestGormAggregate_BucketByMonth verifies month truncation via strftime.
func TestGormAggregate_BucketByMonth(t *testing.T) {
	db := setupAggDB(t)
	repo := r3gorm.NewGormCRUD[aggRaid, int64](db)
	ctx := context.Background()

	for _, raid := range []aggRaid{
		{Date: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)},
		{Date: time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)},
		{Date: time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC)},
	} {
		_, err := repo.Create(ctx, raid)
		require.NoError(t, err)
	}

	rows, err := repo.Aggregate(ctx, r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("date", r3.BucketMonth, "month")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("month"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	jan, _ := rows[0].Time("month")
	assert.Equal(t, "2026-01-01", jan.Format(time.DateOnly))
	nJan, _ := rows[0].Int64("n")
	assert.Equal(t, int64(2), nJan)
}
