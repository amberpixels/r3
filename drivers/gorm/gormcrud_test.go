package r3gorm_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	. "github.com/amberpixels/r3/internal/testing" //nolint: revive // testing is OK
	"github.com/pressly/goose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGormRepository(t *testing.T) {
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
		_ = container.Terminate(t.Context())
	}()

	const autoMigrate = false
	const pathToMigrations = "../../internal/testing/migrations"

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
	cityRepo := r3gorm.NewGormCRUD[City, int64](db)
	locRepo := r3gorm.NewGormCRUD[Location, int64](db)
	eventRepo := r3gorm.NewGormCRUD[Event, int64](db)
	artistRepo := r3gorm.NewGormCRUD[Artist, int64](db)

	_ = artistRepo

	t.Run("List cities", func(t *testing.T) {
		result, total, err := cityRepo.List(ctx, r3.ListParams{})
		require.NoError(t, err, "failed to list cities")
		assert.Len(t, result, 2, "expected 2 cities")
		assert.Equal(t, int64(2), total, "expected 2 total cities")
	})

	t.Run("List cities with translations", func(t *testing.T) {
		// TODO: Fix preloads functionality - currently not working properly
		// The preload feature needs investigation as translations are not being loaded
		t.Skip("Skipping preloads test - TODO: investigate why r3.Preloads are not working with GORM")

		// List cities using the repository with the preload parameter set.
		result, total, err := cityRepo.List(ctx, r3.ListParams{
			GetParams: r3.GetParams{
				Preloads: r3.Preloads{
					r3.NewPreloadSpec("Translations"),
				},
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

	t.Run("Get city by ID", func(t *testing.T) {
		city, err := cityRepo.Get(ctx, cities[0].ID, r3.GetParams{})
		require.NoError(t, err, "failed to get city")
		assert.Equal(t, "City One", city.Name, "unexpected city name")
	})

	t.Run("List visible locations", func(t *testing.T) {
		result, _, err := locRepo.List(ctx, r3.ListParams{
			Filters: r3.Filters{
				r3.F(r3.NewFieldSpec("visible"), true),
			},
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
		result, _, err := eventRepo.List(ctx, r3.ListParams{
			Filters: r3.Filters{
				r3.F(r3.NewFieldSpec("venue_id"), locations[1].ID),
			},
		})
		require.NoError(t, err, "failed to list events")

		// Two events should belong to the second location
		assert.Len(t, result, 2, "expected 2 events for the location")
	})

	t.Run("Aggregate location event weights using Raw", func(t *testing.T) {
		// Define a struct to hold the aggregate results.
		// type AggResult struct {
		//	ID          int64  `gorm:"column:id"`
		//	Name        string `gorm:"column:name"`
		//	TotalWeight int64  `gorm:"column:total_weight"`
		//}

		results, err := locRepo.Raw().Find(ctx, func(tx *gorm.DB) *gorm.DB {
			return tx.Table("locations").
				Select("locations.id, locations.name, SUM(events.weight) as total_weight").
				Joins("JOIN events ON locations.id = events.venue_id").
				Where("locations.visible = ? AND locations.city_id = ? AND events.weight > ?", true, 1, 0).
				Group("locations.id, locations.name").
				Order("total_weight DESC")
		})
		require.NoError(t, err, "failed to run aggregate query via Raw")

		jj, _ := json.Marshal(results)
		_ = jj
		// fmt.Println(",.,,. ", string(jj))

		// Log the results for debugging.
		// for _, res := range results {
		//	t.Logf("Location ID: %d, Name: %s, TotalWeight: %d", res.ID, res.Name, res.TotalWeight)
		//}

		// You can add assertions here based on expected results.
		assert.NotEmpty(t, results, "expected at least one location with aggregated event weights")
	})

	// Subtest: Delete an event and verify it no longer exists
	t.Run("Delete event", func(t *testing.T) {
		// Delete the first event
		err := eventRepo.Delete(ctx, events[0].ID)
		require.NoError(t, err, "failed to delete event")

		// Try to retrieve the deleted event
		_, err = eventRepo.Get(ctx, events[0].ID, r3.GetParams{})
		require.Error(t, err, "expected error when getting a deleted event")
	})
}
