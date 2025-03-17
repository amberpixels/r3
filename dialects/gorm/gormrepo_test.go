package depogorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/amberpixels/depo"
	depogorm "github.com/amberpixels/depo/dialects/gorm"
	. "github.com/amberpixels/depo/testing"
	"github.com/pressly/goose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGormRepository(t *testing.T) {
	ctx := context.Background()

	// Set up the PostgreSQL container
	container, db, err := setupPostgresContainer()
	require.NoError(t, err, "failed to start container")
	defer func() {
		_ = container.Terminate(context.Background())
	}()

	const autoMigrate = false
	const pathToMigrations = "../../testing/migrations"

	// Migrate the new models
	if autoMigrate {
		err = db.AutoMigrate(&City{}, &Location{}, &Event{}, &Artist{}, &ArtistToEvent{})
		require.NoError(t, err, "failed to migrate models")
	} else {
		// Get the underlying sql.DB from gorm.DB.
		sqlDB, err := db.DB()
		require.NoError(t, err, "failed to get underlying sql.DB")

		// Run migrations using Goose.
		// Assumes that your migration scripts are located in the "./migrations" directory.
		err = goose.Up(sqlDB, pathToMigrations)
		require.NoError(t, err, "failed to run migrations")

		defer func() {
			// Run down migrations
			if err := goose.Down(sqlDB, pathToMigrations); err != nil {
				t.Logf("failed to run down migration: %v", err)
			}
		}()
	}

	// Create repositories for each model
	cityRepo := depogorm.NewGormRepository[City, int64](db)
	locRepo := depogorm.NewGormRepository[Location, int64](db)
	eventRepo := depogorm.NewGormRepository[Event, int64](db)
	artistRepo := depogorm.NewGormRepository[Artist, int64](db)

	// Pre-fill test data

	// Create 2 cities
	cities := []City{
		{Name: "City One", CountryName: "Country A", Popularity: 50},
		{Name: "City Two", CountryName: "Country B", Popularity: 70},
	}
	for i, city := range cities {
		created, err := cityRepo.Create(ctx, city)
		require.NoError(t, err, "failed to create city")
		cities[i] = created
	}

	// Create 4 locations (2 per city)
	locations := []Location{
		{Name: "Location 1", Slug: "loc1", CityID: cities[0].ID, Popularity: 10, Visible: true},
		{Name: "Location 2", Slug: "loc2", CityID: cities[0].ID, Popularity: 20, Visible: true},
		{Name: "Location 3", Slug: "loc3", CityID: cities[1].ID, Popularity: 30, Visible: false},
		{Name: "Location 4", Slug: "loc4", CityID: cities[1].ID, Popularity: 40, Visible: true},
	}
	for i, loc := range locations {
		created, err := locRepo.Create(ctx, loc)
		require.NoError(t, err, "failed to create location")
		locations[i] = created
	}

	// Create 8 events (2 per location)
	var events []Event
	now := time.Now()
	for _, loc := range locations {
		for j := range 2 {
			event := Event{
				HappenedAt: now.Add(time.Duration(j) * time.Hour),
				Weight:     100 + j,
				VenueID:    loc.ID,
				Active:     true,
			}
			created, err := eventRepo.Create(ctx, event)
			require.NoError(t, err, "failed to create event")
			events = append(events, created)
		}
	}

	// Create 3 artists.
	artists := []Artist{
		{Name: "David Bowie"},
		{Name: "Michael C. Hall"},
		{Name: "Thom Yorke"},
	}
	for i, artist := range artists {
		created, err := artistRepo.Create(ctx, artist)
		require.NoError(t, err, "failed to create artist")
		artists[i] = created
	}

	// Associate artists with events.
	// For even-indexed events, assign Artist One and Artist Two.
	// For odd-indexed events, assign Artist Two and Artist Three.
	for i, event := range events {
		var assocArtists []Artist
		if i%2 == 0 {
			assocArtists = []Artist{artists[0], artists[1]}
		} else {
			assocArtists = []Artist{artists[1], artists[2]}
		}
		// Use GORM's Association Mode to append the artists.
		err := db.Model(&event).Association("Artists").Append(assocArtists)
		require.NoError(t, err, "failed to associate artists with event")
	}

	// Subtest: List cities
	t.Run("List cities", func(t *testing.T) {
		result, err := cityRepo.List(ctx, depo.ListParams{})
		require.NoError(t, err, "failed to list cities")
		assert.Len(t, result, 2, "expected 2 cities")
	})

	t.Run("Get city by ID", func(t *testing.T) {
		city, err := cityRepo.Get(ctx, cities[0].ID, depo.GetParams{})
		require.NoError(t, err, "failed to get city")
		assert.Equal(t, "City One", city.Name, "unexpected city name")
	})

	t.Run("List visible locations", func(t *testing.T) {
		result, err := locRepo.List(ctx, depo.ListParams{
			Filters: depo.NewFiltersGroup().WhereTrue("visible"),
		})
		require.NoError(t, err, "failed to list locations")

		// Expected locations: 1,2 and 4 are visible (3 is not)
		assert.Len(t, result, 3, "expected 3 visible locations")
	})

	// Subtest: Update a location's popularity
	t.Run("Update location", func(t *testing.T) {
		loc := locations[0]
		loc.Popularity = 99
		updated, err := locRepo.Update(ctx, loc)
		require.NoError(t, err, "failed to update location")
		assert.Equal(t, 99, updated.Popularity, "location popularity not updated")
	})

	t.Run("List events for a location", func(t *testing.T) {
		result, err := eventRepo.List(ctx, depo.ListParams{
			Filters: depo.NewFiltersGroup().WhereEq("venue_id", locations[1].ID),
		})
		require.NoError(t, err, "failed to list events")

		// Two events should belong to the second location
		assert.Len(t, result, 2, "expected 2 events for the location")
	})

	// Subtest: Verify artists are associated with events.
	t.Run("Verify event artist associations", func(t *testing.T) {
		// Reload an event to check its associated artists.
		var testEvent Event
		err := db.Preload("Artists").First(&testEvent, events[0].ID).Error
		require.NoError(t, err, "failed to preload event artists")
		// For an even-indexed event, we expect 2 artists: Artist One and Artist Two.
		assert.Len(t, testEvent.Artists, 2, "expected 2 associated artists")
		// Verify the expected artist names.
		names := []string{testEvent.Artists[0].Name, testEvent.Artists[1].Name}
		assert.Contains(t, names, "David Bowie")
		assert.Contains(t, names, "Michael C. Hall")
	})

	// Subtest: Delete an event and verify it no longer exists
	t.Run("Delete event", func(t *testing.T) {
		// Delete the first event
		err := eventRepo.Delete(ctx, events[0].ID)
		require.NoError(t, err, "failed to delete event")

		// Try to retrieve the deleted event
		_, err = eventRepo.Get(ctx, events[0].ID, depo.GetParams{})
		assert.Error(t, err, "expected error when getting a deleted event")
	})
}
