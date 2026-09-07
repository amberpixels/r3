package r3mongo_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Setting is a Mongo-backed entity with a caller-supplied string _id and a
// unique "key" field, exercising upsert both on the PK and on a unique field.
type Setting struct {
	ID    string `bson:"_id"`
	Key   string `bson:"key"`
	Value string `bson:"value"`
	Notes string `bson:"notes"`
}

func TestMongoUpsertAndBulkPatch(t *testing.T) {
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

	t.Run("Upsert inserts then updates on PK conflict", func(t *testing.T) {
		repo := r3mongo.NewMongoCRUD[Setting, string](db.Collection("upsert_pk"))

		// Insert branch: a fresh row keyed by its string PK.
		got, err := repo.Upsert(ctx, Setting{ID: "theme", Key: "theme", Value: "dark", Notes: "initial"})
		require.NoError(t, err)
		assert.Equal(t, "dark", got.Value)
		assert.Equal(t, "initial", got.Notes)

		n, err := repo.Count(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), n, "insert branch adds exactly one row")

		// Update branch: same PK collides and every field is replaced.
		got, err = repo.Upsert(ctx, Setting{ID: "theme", Key: "theme", Value: "light", Notes: "changed"})
		require.NoError(t, err)
		assert.Equal(t, "light", got.Value)
		assert.Equal(t, "changed", got.Notes)

		n, err = repo.Count(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), n, "update branch does not add a row")
	})

	t.Run("Upsert on a unique field with partial UpdateFields", func(t *testing.T) {
		repo := r3mongo.NewMongoCRUD[Setting, string](db.Collection("upsert_unique"))

		// Insert branch: _id is generated (caller left it empty), conflict target is "key".
		first, err := repo.Upsert(ctx, Setting{ID: "s1", Key: "locale", Value: "en", Notes: "n1"},
			r3.OnConflict("key"))
		require.NoError(t, err)
		assert.Equal(t, "en", first.Value)

		// Update branch: same "key" collides; only Value is written, Notes is preserved.
		second, err := repo.Upsert(ctx,
			Setting{ID: "s2", Key: "locale", Value: "fr", Notes: "ignored"},
			r3.OnConflict("key"),
			r3.UpdateOnConflict(r3.NewFieldSpec("value")),
		)
		require.NoError(t, err)
		assert.Equal(t, first.ID, second.ID, "same document is updated, not a new one")
		assert.Equal(t, "fr", second.Value, "the restricted field is written")
		assert.Equal(t, "n1", second.Notes, "fields outside UpdateFields are preserved")

		n, err := repo.Count(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), n)
	})

	t.Run("Upsert with a zero PK generates the _id", func(t *testing.T) {
		type Doc struct {
			ID   bson.ObjectID `bson:"_id"`
			Name string        `bson:"name"`
		}
		repo := r3mongo.NewMongoCRUD[Doc, bson.ObjectID](db.Collection("upsert_genid"))

		a, err := repo.Upsert(ctx, Doc{Name: "a"})
		require.NoError(t, err)
		require.False(t, a.ID.IsZero(), "a zero _id must be generated on insert")

		b, err := repo.Upsert(ctx, Doc{Name: "b"})
		require.NoError(t, err)
		require.False(t, b.ID.IsZero())
		assert.NotEqual(t, a.ID, b.ID, "zero-PK upserts must not collide onto one document")

		n, err := repo.Count(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
	})

	t.Run("PatchWhere updates matching rows and reports the count", func(t *testing.T) {
		repo := r3mongo.NewMongoCRUD[Setting, string](db.Collection("bulk_patch"))
		for _, s := range []Setting{
			{ID: "a", Key: "grp1", Value: "x", Notes: "n"},
			{ID: "b", Key: "grp1", Value: "y", Notes: "n"},
			{ID: "c", Key: "grp2", Value: "z", Notes: "n"},
		} {
			_, err := repo.Upsert(ctx, s) // Upsert honors the caller-supplied string _id
			require.NoError(t, err)
		}

		affected, err := repo.PatchWhere(ctx,
			r3.Filters{r3.Eq("key", "grp1")},
			Setting{Value: "patched"},
			r3.Fields{r3.NewFieldSpec("value")},
		)
		require.NoError(t, err)
		assert.Equal(t, int64(2), affected, "only the two grp1 rows are affected")

		got, _, err := repo.List(ctx, r3.Query{Filters: r3.Filters{r3.Eq("value", "patched")}})
		require.NoError(t, err)
		assert.Len(t, got, 2)

		// The unaffected row keeps its value.
		c, err := repo.Get(ctx, "c")
		require.NoError(t, err)
		assert.Equal(t, "z", c.Value)
	})

	t.Run("PatchWhere rejects an invalid field", func(t *testing.T) {
		repo := r3mongo.NewMongoCRUD[Setting, string](db.Collection("bulk_patch_invalid"))
		_, err := repo.PatchWhere(ctx,
			r3.Filters{r3.Eq("key", "grp1")},
			Setting{},
			r3.Fields{r3.NewFieldSpec("nonexistent")},
		)
		require.ErrorIs(t, err, r3.ErrInvalidPatchField)
	})
}
