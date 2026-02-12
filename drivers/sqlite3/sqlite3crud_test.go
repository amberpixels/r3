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
