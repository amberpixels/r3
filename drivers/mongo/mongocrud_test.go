package r3mongo_test

import (
	"testing"
	"time"

	"github.com/amberpixels/k1/maybe"
	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// City is a simple Mongo-backed entity. String _id is set explicitly so the
// engine never has to coerce a generated ObjectID into a string field.
type City struct {
	ID         string `bson:"_id"`
	Name       string `bson:"name"`
	Country    string `bson:"country"`
	Popularity int    `bson:"popularity"`
}

// SoftDoc exercises soft-delete with a NON-POINTER time.Time field — the M6
// regression target. A live record stores the zero time (a zero BSON Date, not
// null), so the "not deleted" filter must match it. The bson tag supplies the
// stored name; the r3 tag supplies the soft_delete flag (they agree on the name).
type SoftDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	DeletedAt time.Time `bson:"deleted_at" r3:"soft_delete"`
}

func TestMongoRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available — Mongo integration test requires Docker")
	}

	ctx := t.Context()
	db, cleanup, err := setupMongoContainer(ctx)
	if err != nil {
		t.Skipf("Failed to set up Mongo container: %v", err)
	}
	defer cleanup()

	cityRepo := r3mongo.NewMongoCRUD[City, string](db.Collection("cities"))

	// Seed three cities with explicit string ids.
	seed := []City{
		{ID: "c1", Name: "Berlin", Country: "Germany", Popularity: 100},
		{ID: "c2", Name: "Paris", Country: "France", Popularity: 90},
		{ID: "c3", Name: "Munich", Country: "Germany", Popularity: 50},
	}
	for _, c := range seed {
		_, err := cityRepo.Create(ctx, c)
		require.NoError(t, err, "seed %s", c.ID)
	}

	t.Run("Get by id", func(t *testing.T) {
		got, err := cityRepo.Get(ctx, "c1")
		require.NoError(t, err)
		assert.Equal(t, "Berlin", got.Name)
		assert.Equal(t, 100, got.Popularity)
	})

	t.Run("Get missing returns ErrNotFound", func(t *testing.T) {
		_, err := cityRepo.Get(ctx, "nope")
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("List and Count", func(t *testing.T) {
		all, total, err := cityRepo.List(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Len(t, all, 3)
		assert.Equal(t, int64(3), total)

		n, err := cityRepo.Count(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
	})

	t.Run("filter Eq", func(t *testing.T) {
		got, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.F(r3.NewFieldSpec("country"), "Germany")},
		})
		require.NoError(t, err)
		assert.Len(t, got, 2, "Berlin and Munich are in Germany")
	})

	t.Run("filter In and NotIn", func(t *testing.T) {
		in, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.In("name", []string{"Berlin", "Paris"})},
		})
		require.NoError(t, err)
		assert.Len(t, in, 2)

		notIn, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.NotIn("name", []string{"Berlin"})},
		})
		require.NoError(t, err)
		assert.Len(t, notIn, 2, "everything except Berlin")
	})

	t.Run("filter Gt", func(t *testing.T) {
		got, _, err := cityRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Fop(r3.NewFieldSpec("popularity"), r3.OperatorGt, 60)},
		})
		require.NoError(t, err)
		assert.Len(t, got, 2, "Berlin (100) and Paris (90)")
	})

	t.Run("sort ascending and descending", func(t *testing.T) {
		asc, _, err := cityRepo.List(ctx, r3.Query{
			Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("popularity"))},
		})
		require.NoError(t, err)
		require.Len(t, asc, 3)
		assert.Equal(t, []int{50, 90, 100},
			[]int{asc[0].Popularity, asc[1].Popularity, asc[2].Popularity})

		desc, _, err := cityRepo.List(ctx, r3.Query{
			Sorts: r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("popularity"))},
		})
		require.NoError(t, err)
		require.Len(t, desc, 3)
		assert.Equal(t, []int{100, 90, 50},
			[]int{desc[0].Popularity, desc[1].Popularity, desc[2].Popularity})
	})

	t.Run("Update", func(t *testing.T) {
		c, err := cityRepo.Get(ctx, "c2")
		require.NoError(t, err)
		c.Popularity = 95
		_, err = cityRepo.Update(ctx, c)
		require.NoError(t, err)

		got, err := cityRepo.Get(ctx, "c2")
		require.NoError(t, err)
		assert.Equal(t, 95, got.Popularity)
	})

	t.Run("Update missing returns ErrNotFound", func(t *testing.T) {
		_, err := cityRepo.Update(ctx, City{ID: "ghost", Name: "X"})
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("Patch returns the full entity", func(t *testing.T) {
		patched, err := cityRepo.Patch(ctx, City{ID: "c1", Popularity: 111},
			r3.Fields{r3.NewFieldSpec("popularity")})
		require.NoError(t, err)
		assert.Equal(t, 111, patched.Popularity)
		assert.Equal(t, "Berlin", patched.Name, "unpatched fields come back populated")
	})

	t.Run("Patch missing returns ErrNotFound", func(t *testing.T) {
		_, err := cityRepo.Patch(ctx, City{ID: "ghost", Popularity: 1},
			r3.Fields{r3.NewFieldSpec("popularity")})
		require.ErrorIs(t, err, r3.ErrNotFound)
	})

	t.Run("Delete and Delete missing", func(t *testing.T) {
		require.NoError(t, cityRepo.Delete(ctx, "c3"))
		_, err := cityRepo.Get(ctx, "c3")
		require.ErrorIs(t, err, r3.ErrNotFound)

		require.ErrorIs(t, cityRepo.Delete(ctx, "c3"), r3.ErrNotFound, "second delete is not found")
	})

	t.Run("soft-delete with non-pointer time.Time (M6) + ErrNotFound (H2)", func(t *testing.T) {
		softRepo := r3mongo.NewMongoCRUD[SoftDoc, string](db.Collection("soft_docs"))

		_, err := softRepo.Create(ctx, SoftDoc{ID: "s1", Name: "alive"})
		require.NoError(t, err)

		// M6: a live record stores the zero time (not null) and MUST be visible.
		got, err := softRepo.Get(ctx, "s1")
		require.NoError(t, err, "live record with zero deleted_at must be found (M6)")
		assert.Equal(t, "alive", got.Name)

		list, total, err := softRepo.List(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Len(t, list, 1, "live record must appear in List (M6)")
		assert.Equal(t, int64(1), total)

		// Soft-delete hides it.
		require.NoError(t, softRepo.Delete(ctx, "s1"))
		_, err = softRepo.Get(ctx, "s1")
		require.ErrorIs(t, err, r3.ErrNotFound, "soft-deleted record must be hidden")

		empty, total, err := softRepo.List(ctx, r3.Query{})
		require.NoError(t, err)
		assert.Empty(t, empty)
		assert.Equal(t, int64(0), total)

		// IncludeTrashed surfaces it again.
		trashed, err := softRepo.Get(ctx, "s1", r3.Query{IncludeTrashed: maybe.Some(true)})
		require.NoError(t, err)
		assert.Equal(t, "alive", trashed.Name)

		// Restore makes it live again.
		require.NoError(t, softRepo.Restore(ctx, "s1"))
		restored, err := softRepo.Get(ctx, "s1")
		require.NoError(t, err, "restored record must be visible again")
		assert.Equal(t, "alive", restored.Name)

		// H2: soft-delete / restore of a missing id returns ErrNotFound.
		require.ErrorIs(t, softRepo.Delete(ctx, "missing"), r3.ErrNotFound)
		require.ErrorIs(t, softRepo.Restore(ctx, "missing"), r3.ErrNotFound)
	})
}
