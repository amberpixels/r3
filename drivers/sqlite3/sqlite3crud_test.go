package r3sqlite3_test

import (
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
	"github.com/pressly/goose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SQLite-specific test models ---
// These use `db` struct tags for column mapping with the raw SQL driver.
// Relation fields (pointers, slices) are omitted since there's no ORM layer.

// City represents a geographical city.
type City struct {
	ID          int64  `db:"id,pk"`
	Name        string `db:"name"`
	CountryName string `db:"country_name"`
	Popularity  int    `db:"popularity"`
}

// Location represents a location that belongs to a city.
type Location struct {
	ID         int64  `db:"id,pk"`
	Name       string `db:"name"`
	Slug       string `db:"slug"`
	CityID     int64  `db:"city_id"`
	Popularity int    `db:"popularity"`
	Visible    bool   `db:"visible"`
}

// Event represents an event associated with a location.
type Event struct {
	ID         int64     `db:"id,pk"`
	HappenedAt time.Time `db:"happened_at"`
	Name       *string   `db:"name"`
	Weight     int       `db:"weight"`
	VenueID    int64     `db:"venue_id"`
	Active     bool      `db:"active"`
}

// Artist represents a person who performs at events.
type Artist struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
}

func TestSqlite3Repository(t *testing.T) {
	ctx := t.Context()

	// Set up the in-memory SQLite database
	db, err := setupSQLiteDB()
	require.NoError(t, err, "failed to setup SQLite database")
	defer db.Close()

	const pathToMigrations = "../../internal/testing/migrations_sqlite"

	// Set goose dialect to sqlite3 (default is postgres).
	err = goose.SetDialect("sqlite3")
	require.NoError(t, err, "failed to set goose dialect")

	// Run migrations using Goose.
	err = goose.Up(db, pathToMigrations)
	require.NoError(t, err, "failed to run migrations")

	defer func() {
		// Run down migrations
		if err := goose.Down(db, pathToMigrations); err != nil {
			t.Logf("failed to run down migration: %v", err)
		}
	}()

	// Create repositories for each model
	cityRepo := r3sqlite3.NewSqlite3CRUD[City, int64](db)
	locRepo := r3sqlite3.NewSqlite3CRUD[Location, int64](db)
	eventRepo := r3sqlite3.NewSqlite3CRUD[Event, int64](db)
	artistRepo := r3sqlite3.NewSqlite3CRUD[Artist, int64](db)

	_ = artistRepo

	// Verify initial data
	t.Run("List cities", func(t *testing.T) {
		result, total, err := cityRepo.List(ctx, r3.Query{})
		require.NoError(t, err, "failed to list cities")
		assert.Len(t, result, 2, "expected 2 cities")
		assert.Equal(t, int64(2), total, "expected 2 total cities")
	})

	t.Run("Get city by ID", func(t *testing.T) {
		city, err := cityRepo.Get(ctx, int64(1), r3.Query{})
		require.NoError(t, err, "failed to get city")
		assert.Equal(t, "City One", city.Name, "unexpected city name")
	})

	t.Run("Get missing city returns ErrNotFound", func(t *testing.T) {
		_, err := cityRepo.Get(ctx, int64(99999))
		require.Error(t, err)
		assert.ErrorIs(t, err, r3.ErrNotFound, "missing record must normalize to r3.ErrNotFound")
	})

	t.Run("Count cities", func(t *testing.T) {
		n, err := cityRepo.Count(ctx)
		require.NoError(t, err, "failed to count cities")
		assert.Equal(t, int64(2), n, "expected 2 cities")
	})

	t.Run("Count with filter ignores pagination", func(t *testing.T) {
		// A page size of 1 must not cap the count.
		n, err := cityRepo.Count(ctx, r3.Query{
			Pagination: r3.NewPaginationSpec(1, 1),
			Filters:    r3.Filters{r3.Eq("name", "City One")},
		})
		require.NoError(t, err, "failed to count cities with filter")
		assert.Equal(t, int64(1), n, "expected 1 matching city")
	})

	// The p44-shaped stats query: GROUP BY a key, COUNT + MIN/MAX, HAVING,
	// ordered by an aggregate alias. Seeded events per venue:
	// 1→(101,106), 2→(102,107), 3→(103,108), 4→(104), 5→(105).
	t.Run("Aggregate events per venue", func(t *testing.T) {
		rows, err := eventRepo.Aggregate(ctx, r3.Query{
			GroupBy: r3.GroupBy("venue_id"),
			Aggregates: r3.Aggregates{
				r3.AggCount("n"),
				r3.AggMin("weight", "min_weight"),
				r3.AggMax("weight", "max_weight"),
				r3.AggMax("happened_at", "last_at"),
			},
			Having: r3.Filters{r3.Gt("n", 1)},
			Sorts:  r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("venue_id"))},
		})
		require.NoError(t, err, "failed to aggregate events")
		require.Len(t, rows, 3, "venues 1-3 have two events each")

		venue, ok := rows[0].Int64("venue_id")
		require.True(t, ok)
		assert.Equal(t, int64(1), venue)

		n, _ := rows[0].Int64("n")
		minW, _ := rows[0].Int64("min_weight")
		maxW, _ := rows[0].Int64("max_weight")
		assert.Equal(t, int64(2), n)
		assert.Equal(t, int64(101), minW)
		assert.Equal(t, int64(106), maxW)

		// SQLite returns MAX over a datetime column as TEXT; the accessor
		// must still parse it into a real time.
		lastAt, ok := rows[0].Time("last_at")
		require.True(t, ok, "Time() must parse SQLite's textual MAX(datetime), got %#v", rows[0].Value("last_at"))
		assert.False(t, lastAt.IsZero())
	})

	t.Run("Aggregate whole-set row and IN restriction", func(t *testing.T) {
		rows, err := eventRepo.Aggregate(ctx, r3.Query{
			Filters: r3.Filters{r3.In("venue_id", []int64{4, 5})},
			Aggregates: r3.Aggregates{
				r3.AggCount("n"),
				r3.AggSum("weight", "total"),
				r3.AggCountDistinct("venue_id", "venues"),
			},
		})
		require.NoError(t, err, "failed to aggregate with filter")
		require.Len(t, rows, 1, "no GroupBy = one whole-set row")

		n, _ := rows[0].Int64("n")
		total, _ := rows[0].Int64("total")
		venues, _ := rows[0].Int64("venues")
		assert.Equal(t, int64(2), n)
		assert.Equal(t, int64(209), total, "104 + 105")
		assert.Equal(t, int64(2), venues)
	})

	t.Run("Aggregate sorted by aggregate alias with pagination", func(t *testing.T) {
		rows, err := eventRepo.Aggregate(ctx, r3.Query{
			GroupBy:    r3.GroupBy("venue_id"),
			Aggregates: r3.Aggregates{r3.AggMax("weight", "max_weight")},
			Sorts:      r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("max_weight"))},
			Pagination: r3.NewPaginationSpecWithSize(2),
		})
		require.NoError(t, err, "failed to aggregate sorted by alias")
		require.Len(t, rows, 2, "pagination limits grouped rows")

		first, _ := rows[0].Int64("max_weight")
		second, _ := rows[1].Int64("max_weight")
		assert.Equal(t, int64(108), first, "venue 3 has the heaviest event")
		assert.Equal(t, int64(107), second)
	})

	// C1 regression: IN / NOT IN must expand to one placeholder per value.
	// A single "col IN ?" placeholder bound to a slice is NOT expanded by
	// database/sql and fails at execution, so these must run against a real DB.
	t.Run("List cities with IN filter", func(t *testing.T) {
		both, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.In("name", []string{"City One", "City Two"})},
		})
		require.NoError(t, err, "IN with multiple values must execute")
		assert.Len(t, both, 2, "IN with two values should match both cities")

		one, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.In("name", []string{"City One"})},
		})
		require.NoError(t, err, "IN with a single value must execute")
		assert.Len(t, one, 1, "IN with one value should match one city")

		// An empty IN set collapses to a constant-false predicate, since
		// "col IN ()" is invalid SQL.
		none, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.In("name", []string{})},
		})
		require.NoError(t, err, "empty IN must execute")
		assert.Empty(t, none, "empty IN set should match nothing")
	})

	t.Run("List cities with NOT IN filter", func(t *testing.T) {
		rest, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.NotIn("name", []string{"City One"})},
		})
		require.NoError(t, err, "NOT IN must execute")
		require.Len(t, rest, 1, "NOT IN should exclude the named city")
		assert.Equal(t, "City Two", rest[0].Name)

		// An empty NOT IN set collapses to a constant-true predicate.
		all, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.NotIn("name", []string{})},
		})
		require.NoError(t, err, "empty NOT IN must execute")
		assert.Len(t, all, 2, "empty NOT IN set should match everything")
	})

	t.Run("List locations", func(t *testing.T) {
		result, total, err := locRepo.List(ctx, r3.Query{})
		require.NoError(t, err, "failed to list locations")
		assert.Len(t, result, 5, "expected 5 locations")
		assert.Equal(t, int64(5), total, "expected 5 total locations")
	})

	t.Run("List visible locations", func(t *testing.T) {
		result, _, err := locRepo.List(ctx, r3.Query{
			Filters: r3.Filters{
				r3.F(r3.NewFieldSpec("visible"), true),
			},
		})
		require.NoError(t, err, "failed to list locations")

		// Expected locations: 1,2 and 4,5 are visible (3 is not)
		assert.Len(t, result, 4, "expected 4 visible locations")
	})

	t.Run("List events", func(t *testing.T) {
		result, total, err := eventRepo.List(ctx, r3.Query{})
		require.NoError(t, err, "failed to list events")
		assert.Len(t, result, 8, "expected 8 events")
		assert.Equal(t, int64(8), total, "expected 8 total events")
	})

	t.Run("List events for a location", func(t *testing.T) {
		result, _, err := eventRepo.List(ctx, r3.Query{
			Filters: r3.Filters{
				r3.F(r3.NewFieldSpec("venue_id"), int64(2)),
			},
		})
		require.NoError(t, err, "failed to list events")

		// Two events should belong to location 2
		assert.Len(t, result, 2, "expected 2 events for the location")
	})

	// Subtest: Update a location's popularity
	t.Run("Update location", func(t *testing.T) {
		// First get the location
		loc, err := locRepo.Get(ctx, int64(1), r3.Query{})
		require.NoError(t, err, "failed to get location")

		loc.Popularity = 99
		updated, err := locRepo.Update(ctx, loc)
		require.NoError(t, err, "failed to update location")
		assert.Equal(t, 99, updated.Popularity, "location popularity not updated")

		// Verify by re-fetching
		fetched, err := locRepo.Get(ctx, int64(1), r3.Query{})
		require.NoError(t, err, "failed to re-fetch location")
		assert.Equal(t, 99, fetched.Popularity, "location popularity not persisted")
	})

	t.Run("Raw query", func(t *testing.T) {
		results, err := locRepo.Raw().Query(ctx,
			"SELECT id, name, slug, city_id, popularity, visible FROM locations WHERE visible = ? AND city_id = ? ORDER BY popularity DESC",
			true, 1,
		)
		require.NoError(t, err, "failed to run raw query")
		assert.NotEmpty(t, results, "expected at least one location from raw query")
	})

	// Subtest: Delete an event and verify it no longer exists
	t.Run("Delete event", func(t *testing.T) {
		// Delete the first event
		err := eventRepo.Delete(ctx, int64(1))
		require.NoError(t, err, "failed to delete event")

		// Try to retrieve the deleted event
		_, err = eventRepo.Get(ctx, int64(1), r3.Query{})
		require.Error(t, err, "expected error when getting a deleted event")
	})

	t.Run("Create city", func(t *testing.T) {
		city := City{
			Name:        "City Three",
			CountryName: "Country C",
			Popularity:  30,
		}
		created, err := cityRepo.Create(ctx, city)
		require.NoError(t, err, "failed to create city")
		assert.NotZero(t, created.ID, "expected non-zero ID for created city")
		assert.Equal(t, "City Three", created.Name, "unexpected city name")

		// Verify by fetching
		fetched, err := cityRepo.Get(ctx, created.ID, r3.Query{})
		require.NoError(t, err, "failed to fetch created city")
		assert.Equal(t, "City Three", fetched.Name, "unexpected fetched city name")
	})

	t.Run("List artists", func(t *testing.T) {
		result, total, err := artistRepo.List(ctx, r3.Query{})
		require.NoError(t, err, "failed to list artists")
		assert.Len(t, result, 3, "expected 3 artists")
		assert.Equal(t, int64(3), total, "expected 3 total artists")
	})
}

// TestSqlite3_BackwardCursor verifies keyset pagination in both directions (H1).
// A "before" cursor must return the rows immediately preceding it, in the
// requested order — not the rows farthest from it.
// SoftCity is a soft-deletable model used to exercise soft-delete / Restore
// not-found semantics. It owns its table so the test is self-contained.
type SoftCity struct {
	ID        int64      `db:"id,pk"`
	Name      string     `db:"name"`
	DeletedAt *time.Time `db:"deleted_at,soft_delete"`
}

// TestSqlite3_NotFoundSemantics verifies M2: Update, Patch, Delete, HardDelete
// and Restore all report r3.ErrNotFound when no row matches the primary key, and
// that a no-op update of an existing row still succeeds (not a false ErrNotFound).
func TestSqlite3_NotFoundSemantics(t *testing.T) {
	ctx := t.Context()

	db, err := setupSQLiteDB()
	require.NoError(t, err, "failed to setup SQLite database")
	defer db.Close()

	const pathToMigrations = "../../internal/testing/migrations_sqlite"
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db, pathToMigrations))
	defer func() {
		if err := goose.Down(db, pathToMigrations); err != nil {
			t.Logf("failed to run down migration: %v", err)
		}
	}()

	const missingID = int64(99999)
	cityRepo := r3sqlite3.NewSqlite3CRUD[City, int64](db)

	t.Run("Update missing returns ErrNotFound", func(t *testing.T) {
		_, err := cityRepo.Update(ctx, City{ID: missingID, Name: "ghost", CountryName: "X"})
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("Patch missing returns ErrNotFound", func(t *testing.T) {
		_, err := cityRepo.Patch(ctx, City{ID: missingID}, r3.Fields{r3.NewFieldSpec("name")})
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("Delete missing returns ErrNotFound", func(t *testing.T) {
		err := cityRepo.Delete(ctx, missingID)
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("HardDelete missing returns ErrNotFound", func(t *testing.T) {
		err := cityRepo.HardDelete(ctx, missingID)
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("no-op Update of existing row succeeds", func(t *testing.T) {
		// Migrations seed City One (id 1). Re-applying its own values must NOT be
		// misreported as not-found (the MySQL changed-vs-matched-rows case).
		existing, err := cityRepo.Get(ctx, int64(1))
		require.NoError(t, err)
		_, err = cityRepo.Update(ctx, existing)
		require.NoError(t, err)
	})

	t.Run("soft-delete and Restore missing returns ErrNotFound", func(t *testing.T) {
		_, err := db.ExecContext(ctx,
			`CREATE TABLE soft_cities (id INTEGER PRIMARY KEY, name TEXT, deleted_at DATETIME)`)
		require.NoError(t, err)

		softRepo := r3sqlite3.NewSqlite3CRUD[SoftCity, int64](db)
		created, err := softRepo.Create(ctx, SoftCity{Name: "live"})
		require.NoError(t, err)

		// Soft-delete of a missing row → ErrNotFound.
		require.ErrorIs(t, softRepo.Delete(ctx, missingID), r3.ErrNotFound)
		// Restore of a missing row → ErrNotFound.
		require.ErrorIs(t, softRepo.Restore(ctx, missingID), r3.ErrNotFound)

		// Soft-delete then restore the real row → both succeed.
		require.NoError(t, softRepo.Delete(ctx, created.ID))
		require.NoError(t, softRepo.Restore(ctx, created.ID))
	})
}

func TestSqlite3_BackwardCursor(t *testing.T) {
	ctx := t.Context()

	db, err := setupSQLiteDB()
	require.NoError(t, err, "failed to setup SQLite database")
	defer db.Close()

	const pathToMigrations = "../../internal/testing/migrations_sqlite"
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db, pathToMigrations))
	defer func() {
		if err := goose.Down(db, pathToMigrations); err != nil {
			t.Logf("failed to run down migration: %v", err)
		}
	}()

	cityRepo := r3sqlite3.NewSqlite3CRUD[City, int64](db)

	// Migrations seed City One (id 1) and City Two (id 2). Add three more so the
	// dataset is larger than the page size and the backward bug is observable.
	for _, name := range []string{"City Three", "City Four", "City Five"} {
		_, err := cityRepo.Create(ctx, City{Name: name, CountryName: "X", Popularity: 0})
		require.NoError(t, err)
	}
	// Cities now have ids 1..5.

	sortByID := r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("id"))}

	// Forward sanity check: After id=2, limit 2 → [3, 4].
	afterToken, err := r3.EncodeCursor(r3.CursorValues{"id": int64(2)})
	require.NoError(t, err)
	fwd, _, err := cityRepo.List(ctx, r3.Query{
		Sorts:  sortByID,
		Cursor: r3.NewCursorAfter(afterToken, 2),
	})
	require.NoError(t, err)
	require.Len(t, fwd, 2)
	assert.Equal(t, int64(3), fwd[0].ID)
	assert.Equal(t, int64(4), fwd[1].ID)

	// Backward: Before id=5, limit 2 → the two rows immediately before id 5,
	// in ascending order: [3, 4]. The buggy implementation returned [1, 2].
	beforeToken, err := r3.EncodeCursor(r3.CursorValues{"id": int64(5)})
	require.NoError(t, err)
	bwd, _, err := cityRepo.List(ctx, r3.Query{
		Sorts:  sortByID,
		Cursor: r3.NewCursorBefore(beforeToken, 2),
	})
	require.NoError(t, err)
	require.Len(t, bwd, 2)
	assert.Equal(t, int64(3), bwd[0].ID, "backward cursor should return the rows immediately before, ascending")
	assert.Equal(t, int64(4), bwd[1].ID)
}
