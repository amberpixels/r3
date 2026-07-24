package enginefile_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
)

// seedBucketSessions creates sessions across a fixed set of timestamps so
// day/week/month buckets have predictable counts. All times are UTC.
func seedBucketSessions(t *testing.T) *enginefile.BaseCRUD[Session, int] {
	t.Helper()
	repo := newJSONRepo[Session, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	at := func(s string) time.Time {
		tm, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return tm
	}
	// Mon 2026-01-05, Tue 2026-01-06 (same ISO week), then Mon 2026-01-12
	// (next week), then 2026-02-03 (next month).
	for _, s := range []Session{
		{Label: "a", StartedAt: at("2026-01-05T09:00:00Z")},
		{Label: "b", StartedAt: at("2026-01-05T17:30:00Z")},
		{Label: "c", StartedAt: at("2026-01-06T10:00:00Z")},
		{Label: "d", StartedAt: at("2026-01-12T08:00:00Z")},
		{Label: "e", StartedAt: at("2026-02-03T08:00:00Z")},
	} {
		_, err := repo.Create(ctx, s)
		require.NoError(t, err)
	}
	return repo
}

func TestAggregate_BucketByDay(t *testing.T) {
	repo := seedBucketSessions(t)

	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("started_at", r3.BucketDay, "day")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("day"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 4) // 01-05, 01-06, 01-12, 02-03

	day0, ok := rows[0].Time("day")
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), day0)
	n0, ok := rows[0].Int64("n")
	require.True(t, ok)
	assert.Equal(t, int64(2), n0) // two sessions on 01-05
}

func TestAggregate_BucketByWeekISOMonday(t *testing.T) {
	repo := seedBucketSessions(t)

	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("started_at", r3.BucketWeek, "week")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("week"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 3) // week of 01-05, week of 01-12, week of 02-02

	// The three 01-05/01-06 sessions collapse into the ISO week starting Monday
	// 2026-01-05.
	week0, ok := rows[0].Time("week")
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), week0)
	n0, _ := rows[0].Int64("n")
	assert.Equal(t, int64(3), n0)

	// 2026-02-03 is a Tuesday; its ISO week starts Monday 2026-02-02.
	week2, ok := rows[2].Time("week")
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC), week2)
}

func TestAggregate_BucketByMonthWithGroupBy(t *testing.T) {
	repo := seedBucketSessions(t)

	// A plain group field and a bucket key coexist in one grouped result.
	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("started_at", r3.BucketMonth, "month")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("month"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2) // January, February

	jan, _ := rows[0].Time("month")
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), jan)
	nJan, _ := rows[0].Int64("n")
	assert.Equal(t, int64(4), nJan)

	feb, _ := rows[1].Time("month")
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), feb)
	nFeb, _ := rows[1].Int64("n")
	assert.Equal(t, int64(1), nFeb)
}

func TestAggregate_BucketHavingAndFilter(t *testing.T) {
	repo := seedBucketSessions(t)

	// Having over the bucket's own count, plus a Filter narrowing the input.
	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Filters:    r3.Filters{r3.Gte("started_at", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		Buckets:    r3.Buckets{r3.Bucket("started_at", r3.BucketDay, "day")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Gt("n", 1)},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1) // only 01-05 has >1 session
	day, _ := rows[0].Time("day")
	assert.Equal(t, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), day)
}

func TestAggregate_BucketNonTimeFieldErrors(t *testing.T) {
	repo := seedBucketSessions(t)

	_, err := repo.Aggregate(context.Background(), r3.Query{
		Buckets:    r3.Buckets{r3.Bucket("label", r3.BucketDay, "d")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, r3.ErrInvalidBucket)
}
