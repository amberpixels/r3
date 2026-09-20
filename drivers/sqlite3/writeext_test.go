package r3sqlite3_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
)

// kvSetting is a string-keyed row exercising the raw engine/sql Upsert path
// (INSERT ... ON CONFLICT (key) DO UPDATE ... RETURNING).
type kvSetting struct {
	Key       string    `db:"key,pk"`
	Value     string    `db:"value"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func setupSettings(t *testing.T) *r3sqlite3.Sqlite3CRUD[kvSetting, string] {
	t.Helper()
	db, err := setupSQLiteDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE kv_settings (
		key         TEXT PRIMARY KEY,
		value       TEXT NOT NULL,
		created_at  DATETIME,
		updated_at  DATETIME
	)`)
	require.NoError(t, err)
	return r3sqlite3.NewSqlite3CRUD[kvSetting, string](db)
}

func TestSqlite3Upsert_InsertThenUpdate(t *testing.T) {
	repo := setupSettings(t)
	ctx := context.Background()

	got, err := repo.Upsert(ctx, kvSetting{Key: "theme", Value: "dark"})
	require.NoError(t, err)
	require.Equal(t, "dark", got.Value)
	require.False(t, got.CreatedAt.IsZero(), "created_at set on insert (RETURNING round-trips it)")
	require.False(t, got.UpdatedAt.IsZero())

	time.Sleep(10 * time.Millisecond)

	got2, err := repo.Upsert(ctx, kvSetting{Key: "theme", Value: "light"})
	require.NoError(t, err)
	require.Equal(t, "light", got2.Value)
	require.WithinDuration(t, got.CreatedAt, got2.CreatedAt, time.Millisecond,
		"created_at preserved on the conflict-update branch")
	require.True(t, got2.UpdatedAt.After(got.UpdatedAt), "updated_at bumped on conflict")

	n, err := repo.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

func TestSqlite3Upsert_UpdateOnConflictSubsetAndRejection(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	orig, err := repo.Create(ctx, widget{Title: "a", Slug: "slug-a", Secret: "s1"})
	require.NoError(t, err)

	// Only "title" is written on conflict; "secret" stays.
	_, err = repo.Upsert(ctx, widget{ID: orig.ID, Title: "b", Secret: "nope"},
		r3.OnConflict("id"), r3.UpdateOnConflict(r3.NewFieldSpec("title")))
	require.NoError(t, err)

	got, err := repo.Get(ctx, orig.ID)
	require.NoError(t, err)
	require.Equal(t, "b", got.Title)
	require.Equal(t, "s1", got.Secret)

	// Immutable/readonly columns are rejected from the update set.
	_, err = repo.Upsert(ctx, widget{ID: orig.ID, Slug: "x"},
		r3.OnConflict("id"), r3.UpdateOnConflict(r3.NewFieldSpec("slug")))
	require.ErrorIs(t, err, r3.ErrInvalidPatchField)
}

func TestSqlite3PatchWhere_UpdatesMatching(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	// Seed three rows; two share a title we'll bulk-rewrite.
	for _, title := range []string{"running", "running", "done"} {
		_, err := repo.Create(ctx, widget{Title: title, Slug: "s"})
		require.NoError(t, err)
	}

	n, err := repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("title", "running")},
		widget{Title: "interrupted"},
		r3.Fields{r3.NewFieldSpec("title")},
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	done, err := repo.Count(ctx, r3.Query{Filters: r3.Filters{r3.Eq("title", "done")}})
	require.NoError(t, err)
	require.Equal(t, int64(1), done, "non-matching row untouched")

	interrupted, err := repo.Count(ctx, r3.Query{Filters: r3.Filters{r3.Eq("title", "interrupted")}})
	require.NoError(t, err)
	require.Equal(t, int64(2), interrupted)
}

func TestSqlite3PatchWhere_RejectsNonMutable(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()
	_, err := repo.Create(ctx, widget{Title: "a", Slug: "s"})
	require.NoError(t, err)

	_, err = repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("title", "a")},
		widget{Slug: "x"},
		r3.Fields{r3.NewFieldSpec("slug")}, // immutable
	)
	require.ErrorIs(t, err, r3.ErrInvalidPatchField)
}

// counter is the shape IncrementOnConflict exists for: a key, a running total,
// and a column that is overwritten rather than accumulated on the same write.
type counter struct {
	Key      string `db:"key,pk"`
	Hits     int64  `db:"hits"`
	LastSeen string `db:"last_seen"`
}

func setupCounters(t *testing.T) *r3sqlite3.Sqlite3CRUD[counter, string] {
	t.Helper()
	db, err := setupSQLiteDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE counters (
		key       TEXT PRIMARY KEY,
		hits      INTEGER NOT NULL DEFAULT 0,
		last_seen TEXT
	)`)
	require.NoError(t, err)
	return r3sqlite3.NewSqlite3CRUD[counter, string](db)
}

func TestSqlite3Upsert_IncrementOnConflict(t *testing.T) {
	ctx := context.Background()
	hits := r3.NewFieldSpec("hits")

	t.Run("first write stores the incoming value, later writes add to it", func(t *testing.T) {
		repo := setupCounters(t)

		got, err := repo.Upsert(ctx, counter{Key: "gpt", Hits: 3}, r3.IncrementOnConflict(hits))
		require.NoError(t, err)
		require.Equal(t, int64(3), got.Hits, "insert branch takes the incoming value as-is")

		got, err = repo.Upsert(ctx, counter{Key: "gpt", Hits: 4}, r3.IncrementOnConflict(hits))
		require.NoError(t, err)
		require.Equal(t, int64(7), got.Hits, "conflict branch adds rather than overwrites")

		n, err := repo.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(1), n, "still one row")
	})

	t.Run("a negative increment decrements", func(t *testing.T) {
		repo := setupCounters(t)

		_, err := repo.Upsert(ctx, counter{Key: "gpt", Hits: 10}, r3.IncrementOnConflict(hits))
		require.NoError(t, err)
		got, err := repo.Upsert(ctx, counter{Key: "gpt", Hits: -4}, r3.IncrementOnConflict(hits))
		require.NoError(t, err)
		require.Equal(t, int64(6), got.Hits)
	})

	t.Run("other columns still overwrite on the same write", func(t *testing.T) {
		repo := setupCounters(t)

		_, err := repo.Upsert(ctx, counter{Key: "gpt", Hits: 1, LastSeen: "monday"},
			r3.IncrementOnConflict(hits))
		require.NoError(t, err)
		got, err := repo.Upsert(ctx, counter{Key: "gpt", Hits: 1, LastSeen: "tuesday"},
			r3.IncrementOnConflict(hits))
		require.NoError(t, err)
		require.Equal(t, int64(2), got.Hits, "the counter accumulates")
		require.Equal(t, "tuesday", got.LastSeen, "a non-incremented column is replaced as usual")
	})

	t.Run("a column cannot be both overwritten and incremented", func(t *testing.T) {
		_, err := setupCounters(t).Upsert(ctx, counter{Key: "gpt", Hits: 1},
			r3.UpdateOnConflict(hits),
			r3.IncrementOnConflict(hits),
		)
		require.ErrorIs(t, err, r3.ErrUpsertIncrementConflict)
	})

	t.Run("an unknown increment column is rejected", func(t *testing.T) {
		_, err := setupCounters(t).Upsert(ctx, counter{Key: "gpt"},
			r3.IncrementOnConflict(r3.NewFieldSpec("bogus")))
		require.Error(t, err)
	})
}
