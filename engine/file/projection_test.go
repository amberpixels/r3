package enginefile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
)

// Blobbed is the shape projection exists for: a couple of scalars a listing
// renders, beside a fat column it never reads.
type Blobbed struct {
	ID   int    `json:"id"   r3:"id,pk"`
	Name string `json:"name"`
	Blob string `json:"blob"`
}

func seedBlobbed(t *testing.T) *enginefile.BaseCRUD[Blobbed, int] {
	t.Helper()
	repo := newJSONRepo[Blobbed, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()
	for _, name := range []string{"first", "second"} {
		_, err := repo.Create(ctx, Blobbed{Name: name, Blob: "a very large payload"})
		require.NoError(t, err)
	}
	return repo
}

func TestList_Projection(t *testing.T) {
	ctx := context.Background()

	t.Run("no projection returns every field", func(t *testing.T) {
		got, _, err := seedBlobbed(t).List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "a very large payload", got[0].Blob)
	})

	t.Run("ExcludeFields drops the named column", func(t *testing.T) {
		got, _, err := seedBlobbed(t).List(ctx, r3.Query{ExcludeFields: r3.Exclude("blob")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, row := range got {
			assert.Empty(t, row.Blob, "excluded column must come back zeroed")
			assert.NotEmpty(t, row.Name, "unexcluded columns are untouched")
			assert.NotZero(t, row.ID, "the PK survives every projection")
		}
	})

	t.Run("Fields keeps only what it names, plus the PK", func(t *testing.T) {
		got, _, err := seedBlobbed(t).List(ctx, r3.Query{Fields: r3.Include("name")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, row := range got {
			assert.NotEmpty(t, row.Name)
			assert.Empty(t, row.Blob)
			assert.NotZero(t, row.ID)
		}
	})

	// Filters and sorts read fields a projection may drop, so projecting has to
	// happen after they run - otherwise excluding a column would change which
	// rows come back, not just what they carry.
	t.Run("filtering and sorting on an excluded column still works", func(t *testing.T) {
		got, _, err := seedBlobbed(t).List(ctx, r3.Query{
			Filters:       r3.Filters{r3.Eq("blob", "a very large payload")},
			Sorts:         r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("name"))},
			ExcludeFields: r3.Exclude("blob"),
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "second", got[0].Name)
		assert.Empty(t, got[0].Blob)
	})

	t.Run("both forms at once is a conflict", func(t *testing.T) {
		_, _, err := seedBlobbed(t).List(ctx, r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("blob"),
		})
		assert.ErrorIs(t, err, r3.ErrProjectionConflict)
	})
}

func TestGet_Projection(t *testing.T) {
	ctx := context.Background()
	repo := seedBlobbed(t)

	got, err := repo.Get(ctx, 1, r3.Query{ExcludeFields: r3.Exclude("blob")})
	require.NoError(t, err)
	assert.Equal(t, 1, got.ID)
	assert.NotEmpty(t, got.Name)
	assert.Empty(t, got.Blob)

	// The stored record keeps everything: a projection narrows the answer, never
	// the file.
	full, err := repo.Get(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "a very large payload", full.Blob)
}

// An excluded name that matches no stored field drops nothing. Returning the
// full record for a typo is exactly what the subtractive form exists to prevent,
// so the engine rejects it instead.
func TestProjection_UnknownExcludedField(t *testing.T) {
	ctx := context.Background()
	repo := seedBlobbed(t)

	_, _, err := repo.List(ctx, r3.Query{ExcludeFields: r3.Exclude("bolb")})
	require.ErrorIs(t, err, r3.ErrUnknownField)

	_, err = repo.Get(ctx, 1, r3.Query{ExcludeFields: r3.Exclude("bolb")})
	require.ErrorIs(t, err, r3.ErrUnknownField)
}

// Validation precedes execution: a bad query is a bad query whether or not the
// row happens to exist, so Get must not report ErrNotFound for one.
func TestGet_ProjectionValidatedBeforeLookup(t *testing.T) {
	ctx := context.Background()
	repo := seedBlobbed(t)

	_, err := repo.Get(ctx, 404, r3.Query{
		Fields:        r3.Include("name"),
		ExcludeFields: r3.Exclude("blob"),
	})
	assert.ErrorIs(t, err, r3.ErrProjectionConflict)
}
