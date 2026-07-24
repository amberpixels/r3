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

// Session exercises the time-component filter operators end-to-end.
type Session struct {
	ID        int       `json:"id"         r3:"id,pk"`
	Label     string    `json:"label"`
	StartedAt time.Time `json:"started_at"`
}

func TestList_WeekdayIn(t *testing.T) {
	repo := newJSONRepo[Session, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	// 2026-07-18 is Saturday, 07-19 Sunday, 07-20 Monday, 07-21 Tuesday.
	seed(t, repo, "sat", date(18, 10))
	seed(t, repo, "sun", date(19, 10))
	seed(t, repo, "mon", date(20, 10))
	seed(t, repo, "tue", date(21, 10))

	got, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.WeekdayIn("started_at", time.Saturday, time.Sunday)},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"sat", "sun"}, labels(got))
}

func TestList_TimeOfDayBetween_Wrapping(t *testing.T) {
	repo := newJSONRepo[Session, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	seed(t, repo, "late", date(20, 23)) // 23:00 - in night window
	seed(t, repo, "early", date(20, 3)) // 03:00 - in night window
	seed(t, repo, "noon", date(20, 12)) // 12:00 - out

	got, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		// Nights 22:00-05:00 (wraps midnight).
		Filters: r3.Filters{r3.TimeOfDayBetween("started_at", 22*60, 5*60)},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"late", "early"}, labels(got))
}

func TestList_WeekdayAndTimeOfDay_Composed(t *testing.T) {
	repo := newJSONRepo[Session, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	seed(t, repo, "mon-morning", date(20, 9))  // Monday 09:00 - hit
	seed(t, repo, "mon-evening", date(20, 20)) // Monday 20:00 - wrong time
	seed(t, repo, "sat-morning", date(18, 9))  // Saturday 09:00 - wrong day

	got, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters: r3.Filters{r3.And(
			r3.WeekdayIn("started_at", time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday),
			r3.TimeOfDayBetween("started_at", 8*60, 12*60),
		)},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"mon-morning"}, labels(got))
}

// NullableSession has a nilable timestamp to check NULL handling.
type NullableSession struct {
	ID        int        `json:"id"         r3:"id,pk"`
	Label     string     `json:"label"`
	StartedAt *time.Time `json:"started_at"`
}

func TestList_TimePattern_NullTimeNeverMatchesButDoesNotError(t *testing.T) {
	repo := newJSONRepo[NullableSession, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	sat := date(18, 10) // Saturday
	_, err := repo.Create(ctx, NullableSession{Label: "has-time", StartedAt: &sat})
	require.NoError(t, err)
	_, err = repo.Create(ctx, NullableSession{Label: "null-time", StartedAt: nil})
	require.NoError(t, err)

	got, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.WeekdayIn("started_at", time.Saturday, time.Sunday)},
	})
	// A NULL timestamp must be excluded (SQL semantics), not error the query.
	require.NoError(t, err)
	labels := make([]string, len(got))
	for i, s := range got {
		labels[i] = s.Label
	}
	assert.Equal(t, []string{"has-time"}, labels)
}

func date(d, h int) time.Time {
	return time.Date(2026, time.July, d, h, 0, 0, 0, time.UTC)
}

func seed(t *testing.T, repo *enginefile.BaseCRUD[Session, int], label string, at time.Time) {
	t.Helper()
	_, err := repo.Create(context.Background(), Session{Label: label, StartedAt: at})
	require.NoError(t, err)
}

func labels(sessions []Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.Label
	}
	return out
}
