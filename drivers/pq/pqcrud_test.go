package r3pq_test

import (
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3pq "github.com/amberpixels/r3/drivers/pq"
	"github.com/pressly/goose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- lib/pq-specific test models ---
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

func TestPqRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if Docker is available before attempting to use it
	if !isDockerAvailable() {
		t.Skip(
			"Docker not available - integration test requires Docker to spin up PostgreSQL container. Ensure Docker/OrbStack is running and accessible.",
		)
	}

	ctx := t.Context()

	// Set up the PostgreSQL container
	container, db, err := setupPostgresContainer()
	if err != nil {
		t.Skipf("Failed to setup PostgreSQL container: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = container.Terminate(t.Context())
	}()

	const pathToMigrations = "../../internal/testing/migrations"

	// Run migrations using Goose (database/sql is native here).
	err = goose.Up(db, pathToMigrations)
	require.NoError(t, err, "failed to run migrations")

	defer func() {
		// Run down migrations
		if err := goose.Down(db, pathToMigrations); err != nil {
			t.Logf("failed to run down migration: %v", err)
		}
	}()

	// Create repositories for each model
	cityRepo := r3pq.NewPqCRUD[City, int64](db)
	locRepo := r3pq.NewPqCRUD[Location, int64](db)
	eventRepo := r3pq.NewPqCRUD[Event, int64](db)
	artistRepo := r3pq.NewPqCRUD[Artist, int64](db)

	_ = artistRepo

	// Verify initial data
	t.Run("List cities", func(t *testing.T) {
		result, total, err := cityRepo.List(ctx, r3.Query{})
		require.NoError(t, err, "failed to list cities")
		assert.Len(t, result, 2, "expected 2 cities")
		assert.Equal(t, int64(2), total, "expected 2 total cities")
	})

	// C1 regression: IN / NOT IN must expand to one placeholder per value.
	// A single "col IN ?" bound to a slice is not expanded by the underlying
	// query layer, so this must run against a real database.
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

		all, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.NotIn("name", []string{})},
		})
		require.NoError(t, err, "empty NOT IN must execute")
		assert.Len(t, all, 2, "empty NOT IN set should match everything")
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

	t.Run("Aggregate events per venue", func(t *testing.T) {
		rows, err := eventRepo.Aggregate(ctx, r3.Query{
			GroupBy: r3.GroupBy("venue_id"),
			Aggregates: r3.Aggregates{
				r3.AggCount("n"),
				r3.AggMin("weight", "min_weight"),
				r3.AggMax("weight", "max_weight"),
			},
			Having: r3.Filters{r3.Gt("n", 1)},
			Sorts:  r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("venue_id"))},
		})
		require.NoError(t, err, "failed to aggregate events")
		require.Len(t, rows, 3, "venues 1-3 have two events each")

		venue, ok := rows[0].Int64("venue_id")
		require.True(t, ok, "venue_id must coerce to int64, got %#v", rows[0].Value("venue_id"))
		assert.Equal(t, int64(1), venue)
		n, _ := rows[0].Int64("n")
		minW, _ := rows[0].Int64("min_weight")
		maxW, _ := rows[0].Int64("max_weight")
		assert.Equal(t, int64(2), n)
		assert.Equal(t, int64(101), minW)
		assert.Equal(t, int64(106), maxW)
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
			"SELECT id, name, slug, city_id, popularity, visible FROM locations WHERE visible = $1 AND city_id = $2 ORDER BY popularity DESC",
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
