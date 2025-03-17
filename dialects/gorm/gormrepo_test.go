package depogorm_test

import (
	"context"
	"testing"

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
		err = db.AutoMigrate(
			&City{}, &Location{}, &Event{}, &Artist{}, &ArtistToEvent{},
			&CityTranslation{}, &LocationTranslation{}, &EventTranslation{}, &ArtistToEvent{},
		)
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

	// Raw fetch primary data
	var cities []City
	err = db.Table("cities").Find(&cities).Error
	require.NoError(t, err, "failed to fetch raw cities")
	assert.Len(t, cities, 2, "expected 2 cities")

	var locations []Location
	err = db.Table("locations").Find(&locations).Error
	require.NoError(t, err, "failed to fetch raw locations")
	assert.Len(t, locations, 5, "expected 5 locations")

	// Fetch Events
	var events []Event
	err = db.Table("events").Find(&events).Error
	require.NoError(t, err, "failed to fetch raw events")
	assert.Len(t, events, 8, "expected 8 events")

	// Fetch Artists
	var artists []Artist
	err = db.Table("artists").Find(&artists).Error
	require.NoError(t, err, "failed to fetch raw artists")
	assert.Len(t, artists, 3, "expected 3 artists")

	// Create repositories for each model
	cityRepo := depogorm.NewGormRepository[City, int64](db)
	locRepo := depogorm.NewGormRepository[Location, int64](db)
	eventRepo := depogorm.NewGormRepository[Event, int64](db)
	artistRepo := depogorm.NewGormRepository[Artist, int64](db)

	_ = artistRepo

	t.Run("List cities", func(t *testing.T) {
		result, err := cityRepo.List(ctx, depo.ListParams{})
		require.NoError(t, err, "failed to list cities")
		assert.Len(t, result, 2, "expected 2 cities")
	})

	t.Run("List cities with translations", func(t *testing.T) {

		// List cities using the repository with the preload parameter set.
		result, err := cityRepo.List(ctx, depo.ListParams{
			Preloads: depo.Preloadables{
				depo.NewTablePreload("Translations"),
			},
		})
		require.NoError(t, err, "failed to list cities with translations")
		assert.Len(t, result, 2, "expected 2 cities")

		// Verify that each city has translation records.
		for _, city := range result {
			assert.NotEmpty(t, city.Translations, "expected translations for city")
			assert.Len(t, city.Translations, 3, "expected 3 translations per city (en, es, de)")
		}
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

		// Expected locations: 1,2 and 4,5 are visible (3 is not)
		assert.Len(t, result, 4, "expected 4 visible locations")
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
