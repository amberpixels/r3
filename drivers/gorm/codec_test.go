package r3gorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// codecEvent stores its timestamps as INTEGER unix-seconds via the r3 codec,
// declared with ONLY the r3 tag (no gorm serializer tag). This is the runwell
// layout the field-codec feature exists to support.
type codecEvent struct {
	ID        int        `gorm:"primaryKey" r3:"id,pk"`
	Name      string     `                  r3:"name"`
	StartedAt time.Time  `                  r3:"started_at,codec:unixtime"`
	ExpiresAt *time.Time `                  r3:"expires_at,codec:unixtime"`
}

func (codecEvent) TableName() string { return "codec_events" }

func newCodecRepo(t *testing.T) (*r3gorm.GormCRUD[codecEvent, int], *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`CREATE TABLE codec_events (
			id INTEGER PRIMARY KEY,
			name TEXT,
			started_at INTEGER,
			expires_at INTEGER
		)`,
	).Error)
	return r3gorm.NewGormCRUD[codecEvent, int](db), db
}

func TestGormCodec_RoundTripAndStoredForm(t *testing.T) {
	repo, db := newCodecRepo(t)
	ctx := context.Background()

	start := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	exp := start.Add(24 * time.Hour)
	created, err := repo.Create(ctx, codecEvent{ID: 1, Name: "launch", StartedAt: start, ExpiresAt: &exp})
	require.NoError(t, err)
	assert.True(t, created.StartedAt.Equal(start))

	// Stored columns must be int stamps, not datetimes.
	var rawStart, rawExp int64
	require.NoError(t, db.Raw(`SELECT started_at FROM codec_events WHERE id = 1`).Scan(&rawStart).Error)
	require.NoError(t, db.Raw(`SELECT expires_at FROM codec_events WHERE id = 1`).Scan(&rawExp).Error)
	assert.Equal(t, start.Unix(), rawStart)
	assert.Equal(t, rawExp, exp.Unix())

	// Read back through r3 → domain time.Time in UTC.
	got, err := repo.Get(ctx, 1)
	require.NoError(t, err)
	assert.True(t, got.StartedAt.Equal(start))
	require.NotNil(t, got.ExpiresAt)
	assert.True(t, got.ExpiresAt.Equal(exp))
}

func TestGormCodec_NullZeroPointer(t *testing.T) {
	repo, db := newCodecRepo(t)
	ctx := context.Background()

	// Zero time and nil pointer both store NULL.
	_, err := repo.Create(ctx, codecEvent{ID: 1, Name: "empty"})
	require.NoError(t, err)

	var startNull, expNull bool
	require.NoError(t, db.Raw(`SELECT started_at IS NULL FROM codec_events WHERE id = 1`).Scan(&startNull).Error)
	require.NoError(t, db.Raw(`SELECT expires_at IS NULL FROM codec_events WHERE id = 1`).Scan(&expNull).Error)
	assert.True(t, startNull, "zero time stores NULL")
	assert.True(t, expNull, "nil pointer stores NULL")

	got, err := repo.Get(ctx, 1)
	require.NoError(t, err)
	assert.True(t, got.StartedAt.IsZero())
	assert.Nil(t, got.ExpiresAt)
}

func TestGormCodec_UpdateAndPatch(t *testing.T) {
	repo, db := newCodecRepo(t)
	ctx := context.Background()

	start := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	_, err := repo.Create(ctx, codecEvent{ID: 1, Name: "a", StartedAt: start})
	require.NoError(t, err)

	// Update (full Save) re-encodes the codec'd field.
	moved := start.Add(2 * time.Hour)
	_, err = repo.Update(ctx, codecEvent{ID: 1, Name: "a", StartedAt: moved})
	require.NoError(t, err)
	var raw int64
	require.NoError(t, db.Raw(`SELECT started_at FROM codec_events WHERE id = 1`).Scan(&raw).Error)
	assert.Equal(t, moved.Unix(), raw, "Update stores the re-encoded int")

	// Patch (partial Updates) also re-encodes the codec'd field.
	patched := start.Add(5 * time.Hour)
	got, err := repo.Patch(ctx, codecEvent{ID: 1, StartedAt: patched}, r3.Fields{r3.NewFieldSpec("started_at")})
	require.NoError(t, err)
	assert.True(t, got.StartedAt.Equal(patched))
	require.NoError(t, db.Raw(`SELECT started_at FROM codec_events WHERE id = 1`).Scan(&raw).Error)
	assert.Equal(t, patched.Unix(), raw, "Patch stores the re-encoded int")
}

// seedEvents inserts three events an hour apart and returns their start times.
func seedEvents(t *testing.T, repo *r3gorm.GormCRUD[codecEvent, int]) []time.Time {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	var starts []time.Time
	for i := range 3 {
		s := base.Add(time.Duration(i) * time.Hour)
		_, err := repo.Create(ctx, codecEvent{ID: i + 1, Name: "e", StartedAt: s})
		require.NoError(t, err)
		starts = append(starts, s)
	}
	return starts
}

func TestGormCodec_FilterArgsEncoded(t *testing.T) {
	repo, _ := newCodecRepo(t)
	ctx := context.Background()
	starts := seedEvents(t, repo) // 10:00, 11:00, 12:00

	// Lt with a time.Time bound — the regression that forced runwell to pass a
	// raw int64. r3 must encode the bound to an int for the INTEGER column.
	got, _, err := repo.List(ctx, r3.Query{Filters: r3.Filters{r3.Lt("started_at", starts[2])}})
	require.NoError(t, err)
	assert.Len(t, got, 2, "started_at < 12:00 -> 10:00, 11:00")

	// Gt.
	got, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.Gt("started_at", starts[0])}})
	require.NoError(t, err)
	assert.Len(t, got, 2, "started_at > 10:00 -> 11:00, 12:00")

	// Between (inclusive) with time bounds.
	got, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.Between("started_at", starts[0], starts[1])}})
	require.NoError(t, err)
	assert.Len(t, got, 2, "between 10:00 and 11:00 inclusive")

	// In with a slice of time.Time.
	got, _, err = repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.In("started_at", []time.Time{starts[0], starts[2]})},
	})
	require.NoError(t, err)
	assert.Len(t, got, 2, "in {10:00, 12:00}")
}

func TestGormCodec_Sort(t *testing.T) {
	repo, _ := newCodecRepo(t)
	ctx := context.Background()
	seedEvents(t, repo)

	got, _, err := repo.List(ctx, r3.Query{
		Sorts: r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("started_at"))},
	})
	require.NoError(t, err)
	require.Len(t, got, 3)
	// DESC on the int column orders the same as DESC on the domain time.
	assert.True(t, got[0].StartedAt.After(got[1].StartedAt))
	assert.True(t, got[1].StartedAt.After(got[2].StartedAt))
}

// NOTE: cursor/keyset pagination is a raw-SQL-engine feature; the GORM driver
// implements only offset pagination today (unrelated to codecs). The cursor codec
// encoding lives in the shared engine/sql query-prep and is unit-tested in the
// core package (TestEncodeCursorCodecs); it will be exercised end-to-end when the
// raw-SQL drivers gain codec support in a later increment.
