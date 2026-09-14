package r3mongo_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// PlanStep is a nested value, not an entity of its own.
type PlanStep struct {
	Kind string `bson:"kind"`
	Reps int    `bson:"reps"`
}

// TrainingPlan is GH-22's repro shape: a plain value slice and a slice of
// structs, neither of them a relation. Both used to be dropped on write while the
// bson decoder still populated them on read, so the type round-tripped one way
// only and nothing surfaced until another process re-read the document.
type TrainingPlan struct {
	ID    bson.ObjectID `bson:"_id"`
	Name  string        `bson:"name"`
	Tags  []string      `bson:"tags"`
	Steps []PlanStep    `bson:"steps"`
}

func TestMongoValueSlices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available - Mongo integration test requires Docker")
	}

	ctx := t.Context()
	db, cleanup, err := setupMongoContainer(ctx)
	if err != nil {
		t.Skipf("Failed to set up Mongo container: %v", err)
	}
	defer cleanup()

	coll := db.Collection("training_plans")
	repo := r3mongo.NewMongoCRUD[TrainingPlan, bson.ObjectID](coll)

	created, err := repo.Create(ctx, TrainingPlan{
		Name:  "Base",
		Tags:  []string{"easy", "long"},
		Steps: []PlanStep{{Kind: "warmup", Reps: 3}, {Kind: "main", Reps: 10}},
	})
	require.NoError(t, err)
	require.False(t, created.ID.IsZero())

	t.Run("the stored document carries the value slices", func(t *testing.T) {
		// Read through the driver, not the repo: Create returns the caller's own
		// entity, so every assertion against it passes even when nothing was written.
		var raw bson.M
		require.NoError(t, coll.FindOne(ctx, bson.D{{Key: "_id", Value: created.ID}}).Decode(&raw))
		assert.Contains(t, raw, "tags", "the value slice never reached the document")
		assert.Contains(t, raw, "steps", "the struct slice never reached the document")
	})

	t.Run("a fresh read returns them", func(t *testing.T) {
		got, err := repo.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"easy", "long"}, got.Tags)
		assert.Equal(t, []PlanStep{{Kind: "warmup", Reps: 3}, {Kind: "main", Reps: 10}}, got.Steps)
	})

	t.Run("Update rewrites them", func(t *testing.T) {
		updated := created
		updated.Tags = []string{"tempo"}
		_, err := repo.Update(ctx, updated)
		require.NoError(t, err)

		got, err := repo.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"tempo"}, got.Tags)
	})

	t.Run("Patch names one of them", func(t *testing.T) {
		_, err := repo.Patch(ctx,
			TrainingPlan{ID: created.ID, Steps: []PlanStep{{Kind: "cooldown", Reps: 1}}},
			r3.Fields{r3.NewFieldSpec("steps")})
		require.NoError(t, err)

		got, err := repo.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, []PlanStep{{Kind: "cooldown", Reps: 1}}, got.Steps)
		assert.Equal(t, "Base", got.Name, "patch touched a field it was not given")
	})

	t.Run("a projection can name them", func(t *testing.T) {
		got, err := repo.Get(ctx, created.ID, r3.Query{Fields: r3.Include("tags")})
		require.NoError(t, err)
		assert.Equal(t, []string{"tempo"}, got.Tags)
		assert.Empty(t, got.Name, "an unnamed field survived the projection")
	})
}
