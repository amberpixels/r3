package r3mongo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
)

// Run is the shape projection exists for: a few columns a listing renders,
// beside blobs it never reads. The real case is an activity document whose
// stored provider payloads are 96% of its bytes.
type Run struct {
	ID      bson.ObjectID `bson:"_id,omitempty"`
	Name    string        `bson:"name"`
	Streams string        `bson:"streams_json"`
	Garmin  string        `bson:"garmin_json"`
}

func TestMongoProjection(t *testing.T) {
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

	repo := r3mongo.NewMongoCRUD[Run, bson.ObjectID](db.Collection("projection"))
	stored, err := repo.Create(ctx, Run{Name: "morning", Streams: "big", Garmin: "bigger"})
	require.NoError(t, err)

	t.Run("List with ExcludeFields drops the blobs and keeps the rest", func(t *testing.T) {
		got, _, err := repo.List(ctx, r3.Query{
			ExcludeFields: r3.Exclude("streams_json", "garmin_json"),
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "morning", got[0].Name)
		assert.Equal(t, stored.ID, got[0].ID, "the PK survives every projection")
		assert.Empty(t, got[0].Streams)
		assert.Empty(t, got[0].Garmin)
	})

	t.Run("Get with ExcludeFields does the same", func(t *testing.T) {
		got, err := repo.Get(ctx, stored.ID, r3.Query{ExcludeFields: r3.Exclude("garmin_json")})
		require.NoError(t, err)
		assert.Equal(t, stored.ID, got.ID)
		assert.Equal(t, "big", got.Streams)
		assert.Empty(t, got.Garmin)
	})

	// Mongo rejects a projection mixing 1s and 0s, so r3 refuses the pairing
	// before it reaches the driver.
	t.Run("both projection forms at once is a conflict", func(t *testing.T) {
		_, _, err := repo.List(ctx, r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("garmin_json"),
		})
		assert.ErrorIs(t, err, r3.ErrProjectionConflict)
	})

	t.Run("the stored document keeps everything", func(t *testing.T) {
		full, err := repo.Get(ctx, stored.ID)
		require.NoError(t, err)
		assert.Equal(t, "big", full.Streams)
		assert.Equal(t, "bigger", full.Garmin)
	})
}
