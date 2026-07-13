package r3bun_test

import (
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3bun "github.com/amberpixels/r3/drivers/bun"
	"github.com/pressly/goose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- Bun-specific test models ---
// These use `bun` struct tags.

// City represents a geographical city.
type City struct {
	bun.BaseModel `bun:"table:cities"`

	ID          int64  `bun:"id,pk,autoincrement"`
	Name        string `bun:"name"`
	CountryName string `bun:"country_name"`
	Popularity  int    `bun:"popularity"`

	Translations []CityTranslation `bun:"rel:has-many,join:id=city_id"`
}

// CityTranslation stores the translated name for a City.
type CityTranslation struct {
	bun.BaseModel `bun:"table:city_translations"`

	ID     int64  `bun:"id,pk,autoincrement"`
	Name   string `bun:"name"`
	CityID int64  `bun:"city_id"`
	Locale string `bun:"locale"`
}

// Location represents a location that belongs to a city.
type Location struct {
	bun.BaseModel `bun:"table:locations"`

	ID         int64  `bun:"id,pk,autoincrement"`
	Name       string `bun:"name"`
	Slug       string `bun:"slug"`
	CityID     int64  `bun:"city_id"`
	Popularity int    `bun:"popularity"`
	Visible    bool   `bun:"visible"`

	City         *City                 `bun:"rel:belongs-to,join:city_id=id"`
	Translations []LocationTranslation `bun:"rel:has-many,join:id=location_id"`
}

// LocationTranslation stores the translated name and slug for a Location.
type LocationTranslation struct {
	bun.BaseModel `bun:"table:location_translations"`

	ID         int64  `bun:"id,pk,autoincrement"`
	Name       string `bun:"name"`
	Slug       string `bun:"slug"`
	LocationID int64  `bun:"location_id"`
	Locale     string `bun:"locale"`
}

// Event represents an event associated with a location.
type Event struct {
	bun.BaseModel `bun:"table:events"`

	ID         int64     `bun:"id,pk,autoincrement"`
	HappenedAt time.Time `bun:"happened_at"`
	Name       string    `bun:"name"`
	Weight     int       `bun:"weight"`
	VenueID    int64     `bun:"venue_id"`
	Active     bool      `bun:"active"`

	Translations []EventTranslation `bun:"rel:has-many,join:id=event_id"`
}

// EventTranslation stores the translated name for an Event.
type EventTranslation struct {
	bun.BaseModel `bun:"table:event_translations"`

	ID      int64  `bun:"id,pk,autoincrement"`
	Name    string `bun:"name"`
	EventID int64  `bun:"event_id"`
	Locale  string `bun:"locale"`
}

// Artist represents a person who performs at events.
type Artist struct {
	bun.BaseModel `bun:"table:artists"`

	ID   int64  `bun:"id,pk,autoincrement"`
	Name string `bun:"name"`

	Translations []ArtistTranslation `bun:"rel:has-many,join:id=artist_id"`
}

// ArtistTranslation stores the translated name for an Artist.
type ArtistTranslation struct {
	bun.BaseModel `bun:"table:artist_translations"`

	ID       int64  `bun:"id,pk,autoincrement"`
	Name     string `bun:"name"`
	ArtistID int64  `bun:"artist_id"`
	Locale   string `bun:"locale"`
}

// ArtistToEvent represents the many-to-many relationship between artists and events.
type ArtistToEvent struct {
	bun.BaseModel `bun:"table:artist_to_events"`

	ArtistID int64 `bun:"artist_id,pk"`
	EventID  int   `bun:"event_id,pk"`
}

func TestBunRepository(t *testing.T) {
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
	container, db, sqlDB, err := setupPostgresContainer()
	if err != nil {
		t.Skipf("Failed to setup PostgreSQL container: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = container.Terminate(t.Context())
	}()

	const pathToMigrations = "../../internal/testing/migrations"

	// Run migrations using Goose (Bun wraps database/sql, so sqlDB is available directly).
	err = goose.Up(sqlDB, pathToMigrations)
	require.NoError(t, err, "failed to run migrations")

	defer func() {
		// Run down migrations
		if err := goose.Down(sqlDB, pathToMigrations); err != nil {
			t.Logf("failed to run down migration: %v", err)
		}
	}()

	// Raw fetch primary data to validate the data is there
	var cities []City
	err = db.NewSelect().Model(&cities).Scan(ctx)
	require.NoError(t, err, "failed to fetch raw cities")
	assert.Len(t, cities, 2, "expected 2 cities")

	var locations []Location
	err = db.NewSelect().Model(&locations).Scan(ctx)
	require.NoError(t, err, "failed to fetch raw locations")
	assert.Len(t, locations, 5, "expected 5 locations")

	var events []Event
	err = db.NewSelect().Model(&events).Scan(ctx)
	require.NoError(t, err, "failed to fetch raw events")
	assert.Len(t, events, 8, "expected 8 events")

	var artists []Artist
	err = db.NewSelect().Model(&artists).Scan(ctx)
	require.NoError(t, err, "failed to fetch raw artists")
	assert.Len(t, artists, 3, "expected 3 artists")

	// Create repositories for each model
	cityRepo := r3bun.NewBunCRUD[City, int64](db)
	locRepo := r3bun.NewBunCRUD[Location, int64](db)
	eventRepo := r3bun.NewBunCRUD[Event, int64](db)
	artistRepo := r3bun.NewBunCRUD[Artist, int64](db)

	_ = artistRepo

	t.Run("List cities", func(t *testing.T) {
		result, total, err := cityRepo.List(ctx, r3.Query{})
		require.NoError(t, err, "failed to list cities")
		assert.Len(t, result, 2, "expected 2 cities")
		assert.Equal(t, int64(2), total, "expected 2 total cities")
	})

	t.Run("List cities with translations", func(t *testing.T) {
		// Bun supports Relation for eager loading
		result, total, err := cityRepo.List(ctx, r3.Query{
			Preloads: r3.Preloads{
				r3.NewPreloadSpec("Translations"),
			},
		})
		require.NoError(t, err, "failed to list cities with translations")
		assert.Len(t, result, 2, "expected 2 cities")
		assert.Equal(t, int64(2), total, "expected 2 total cities")

		// Verify that each city has translation records.
		for _, city := range result {
			assert.NotEmpty(t, city.Translations, "expected translations for city")
			assert.Len(t, city.Translations, 3, "expected 3 translations per city (en, es, de)")
		}
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
		city, err := cityRepo.Get(ctx, cities[0].ID, r3.Query{})
		require.NoError(t, err, "failed to get city")
		assert.Equal(t, "City One", city.Name, "unexpected city name")
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

	// Subtest: Update a location's popularity
	t.Run("Patch returns the full DB state", func(t *testing.T) {
		created, err := cityRepo.Create(ctx, City{Name: "PatchCity", CountryName: "Origland", Popularity: 7})
		require.NoError(t, err, "create failed")

		// Patch only the name; CountryName/Popularity are left zero in the input.
		patched, err := cityRepo.Patch(ctx,
			City{ID: created.ID, Name: "PatchedName"},
			r3.Fields{r3.NewFieldSpec("name")},
		)
		require.NoError(t, err, "patch failed")

		assert.Equal(t, "PatchedName", patched.Name, "patched field should be updated")
		// H6: the returned entity must reflect the full DB row, not the sparse
		// input — so the unpatched fields come back populated from the database.
		assert.Equal(t, "Origland", patched.CountryName, "unpatched field must be re-fetched from DB")
		assert.Equal(t, 7, patched.Popularity, "unpatched field must be re-fetched from DB")
	})

	t.Run("Update location", func(t *testing.T) {
		loc := locations[0]
		loc.Popularity = 99
		updated, err := locRepo.Update(ctx, loc)
		require.NoError(t, err, "failed to update location")
		assert.Equal(t, 99, updated.Popularity, "location popularity not updated")
	})

	t.Run("Upsert inserts then updates on conflict", func(t *testing.T) {
		// Insert branch: a fresh row upserts as an insert (RETURNING fills the PK).
		seed, err := cityRepo.Create(ctx, City{Name: "UpsertCity", CountryName: "Origland", Popularity: 1})
		require.NoError(t, err, "seed create failed")
		require.NotZero(t, seed.ID)

		// Conflict branch: same PK, update only the named columns.
		updated, err := r3.UpsertOf(ctx, cityRepo,
			City{ID: seed.ID, Name: "UpsertCityV2", CountryName: "IGNORED", Popularity: 42},
			r3.OnConflict("id"),
			r3.UpdateOnConflict(r3.NewFieldSpec("name"), r3.NewFieldSpec("popularity")),
		)
		require.NoError(t, err, "upsert (conflict) failed")
		assert.Equal(t, seed.ID, updated.ID, "upsert must not change the PK")
		assert.Equal(t, "UpsertCityV2", updated.Name, "conflict must update the name")
		assert.Equal(t, 42, updated.Popularity, "conflict must update popularity")

		// country_name was not in the update set, so the original survives.
		got, err := cityRepo.Get(ctx, seed.ID)
		require.NoError(t, err, "get after upsert failed")
		assert.Equal(t, "Origland", got.CountryName, "non-updated column must be preserved")
	})

	t.Run("List events for a location", func(t *testing.T) {
		result, _, err := eventRepo.List(ctx, r3.Query{
			Filters: r3.Filters{
				r3.F(r3.NewFieldSpec("venue_id"), locations[1].ID),
			},
		})
		require.NoError(t, err, "failed to list events")

		// Two events should belong to the second location
		assert.Len(t, result, 2, "expected 2 events for the location")
	})

	t.Run("Aggregate events per venue (native)", func(t *testing.T) {
		// Compute the expectation from a plain List so the assertion tracks the
		// code-seeded data rather than hard-coded values.
		evs, _, err := eventRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Eq("venue_id", locations[1].ID)},
		})
		require.NoError(t, err, "failed to list venue events")
		require.NotEmpty(t, evs)
		var wantSum, wantMax int64
		for _, e := range evs {
			w := int64(e.Weight)
			wantSum += w
			if w > wantMax {
				wantMax = w
			}
		}

		rows, err := eventRepo.Aggregate(ctx, r3.Query{
			GroupBy: r3.GroupBy("venue_id"),
			Aggregates: r3.Aggregates{
				r3.AggCount("n"),
				r3.AggSum("weight", "total"),
				r3.AggMax("weight", "max_w"),
			},
			Sorts: r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("total"))},
		})
		require.NoError(t, err, "failed to aggregate events")
		require.NotEmpty(t, rows)

		found := false
		for _, row := range rows {
			venue, _ := row.Int64("venue_id")
			if venue != locations[1].ID {
				continue
			}
			found = true
			n, _ := row.Int64("n")
			total, _ := row.Int64("total")
			maxW, _ := row.Int64("max_w")
			assert.Equal(t, int64(len(evs)), n)
			assert.Equal(t, wantSum, total)
			assert.Equal(t, wantMax, maxW)
		}
		assert.True(t, found, "expected an aggregate row for the second location's venue")
	})

	t.Run("Aggregate location event weights using Raw", func(t *testing.T) {
		// Aggregate queries return a different shape than the model,
		// so we use Scan into a dedicated struct.
		type LocationAggregate struct {
			bun.BaseModel `       bun:"table:locations"`
			ID            int64  `bun:"id"`
			Name          string `bun:"name"`
			TotalWeight   int64  `bun:"total_weight"`
		}

		var results []LocationAggregate
		err := locRepo.Raw().Scan(ctx, func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				ColumnExpr("location.id, location.name, SUM(e.weight) as total_weight").
				Join("JOIN events AS e ON e.venue_id = location.id").
				Where("location.visible = ? AND location.city_id = ? AND e.weight > ?", true, 1, 0).
				GroupExpr("location.id, location.name").
				OrderExpr("total_weight DESC")
		}, &results)
		require.NoError(t, err, "failed to run aggregate query via Raw")

		assert.NotEmpty(t, results, "expected at least one location with aggregated event weights")
		// The first result should have the highest total weight
		if len(results) > 0 {
			assert.NotZero(t, results[0].TotalWeight, "expected non-zero total weight")
		}
	})

	t.Run("Raw works inside a transaction", func(t *testing.T) {
		tx, err := cityRepo.BeginTx(ctx)
		require.NoError(t, err, "BeginTx failed")

		// H7: Raw() must be available and bound to the tx (it was nil before the
		// fix, so any use panicked).
		rawer, ok := tx.(interface {
			Raw() *r3bun.BunRaw[City, int64]
		})
		require.True(t, ok, "tx CRUD should expose Raw()")
		require.NotNil(t, rawer.Raw(), "Raw() must not be nil inside a transaction")

		created, err := tx.Create(ctx, City{Name: "TxCity", CountryName: "TxLand", Popularity: 1})
		require.NoError(t, err, "Create in tx failed")

		// Raw runs on the tx connection, so it sees the uncommitted row.
		rows, err := rawer.Raw().Find(ctx, func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("id = ?", created.ID)
		})
		require.NoError(t, err, "Raw query inside tx failed")
		require.Len(t, rows, 1, "Raw inside tx should see the uncommitted row")

		require.NoError(t, tx.Commit(), "Commit failed")

		got, err := cityRepo.Get(ctx, created.ID)
		require.NoError(t, err, "committed city should exist")
		assert.Equal(t, "TxCity", got.Name)
	})

	// Subtest: Delete an event and verify it no longer exists
	t.Run("Delete event", func(t *testing.T) {
		// Delete the first event
		err := eventRepo.Delete(ctx, events[0].ID)
		require.NoError(t, err, "failed to delete event")

		// Try to retrieve the deleted event
		_, err = eventRepo.Get(ctx, events[0].ID, r3.Query{})
		require.Error(t, err, "expected error when getting a deleted event")
	})
}
