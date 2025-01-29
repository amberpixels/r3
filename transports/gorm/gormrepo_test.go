package croodgorm_test

import (
	"context"
	"testing"

	"github.com/amberpixels/crood"
	croodgorm "github.com/amberpixels/crood/transports/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Apple struct {
	ID    int    `gorm:"primaryKey"`
	Name  string `gorm:"not null"`
	Color string `gorm:"not null"`
}

func TestGormRepository(t *testing.T) {
	ctx := context.Background()

	// Set up the PostgreSQL container
	container, db, err := setupPostgresContainer()
	require.NoError(t, err, "failed to start container")
	defer func() {
		_ = container.Terminate(context.Background())
	}()

	// Create the repository
	repo := croodgorm.NewGormRepository[Apple, int](db)

	// Insert sample data
	apples := []Apple{
		{Name: "Golden Delicious", Color: "Yellow"},
		{Name: "Granny Smith", Color: "Green"},
		{Name: "Red Delicious", Color: "Red"},
	}
	for _, apple := range apples {
		_, err := repo.Create(context.Background(), apple)
		require.NoError(t, err, "failed to insert apple")
	}

	// Test List
	t.Run("List apples", func(t *testing.T) {
		results, err := repo.List(ctx, crood.ListParams{})
		require.NoError(t, err, "failed to list apples")

		assert.Len(t, results, 3, "expected 3 apples")

		expected := []Apple{
			{ID: 1, Name: "Golden Delicious", Color: "Yellow"},
			{ID: 2, Name: "Granny Smith", Color: "Green"},
			{ID: 3, Name: "Red Delicious", Color: "Red"},
		}

		for i, result := range results {
			assert.Equal(t, expected[i].ID, result.ID, "unexpected ID")
			assert.Equal(t, expected[i].Name, result.Name, "unexpected Name")
			assert.Equal(t, expected[i].Color, result.Color, "unexpected Color")
		}
	})

	// Test Get
	t.Run("Get apple by ID", func(t *testing.T) {
		result, err := repo.Get(ctx, 1, crood.GetParams{})
		require.NoError(t, err, "failed to get apple")

		assert.Equal(t, 1, result.ID, "unexpected ID")
		assert.Equal(t, "Golden Delicious", result.Name, "unexpected Name")
		assert.Equal(t, "Yellow", result.Color, "unexpected Color")
	})

	// Test Update
	t.Run("Update apple", func(t *testing.T) {
		apple := Apple{ID: 1, Name: "Apple", Color: "Red"}
		result, err := repo.Update(ctx, apple)
		require.NoError(t, err, "failed to update apple")

		assert.Equal(t, 1, result.ID, "unexpected ID")
		assert.Equal(t, "Apple", result.Name, "unexpected Name")
		assert.Equal(t, "Red", result.Color, "unexpected Color")
	})

	// Test Delete
	t.Run("Delete apple", func(t *testing.T) {
		err := repo.Delete(ctx, 1)
		require.NoError(t, err, "failed to delete apple")

		// Ensure it no longer exists
		_, err = repo.Get(ctx, 1, crood.GetParams{})
		assert.Error(t, err, "expected error when getting a deleted apple")
	})
}
