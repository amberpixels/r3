package enginefile_test

import (
	"context"
	"testing"

	"github.com/amberpixels/k1/maybe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
)

func TestCount_All(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})

	n, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func TestCount_WithFilter(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})

	// Count ignores pagination: a page-size of 1 must not cap the count.
	n, err := repo.Count(ctx, r3.Query{
		Pagination: r3.NewPaginationSpec(1, 1),
		Filters:    r3.Filters{r3.Eq("country", "Germany")},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

// With a default page-size cap in place, a per-query Unpaginated() must return
// every row (regression test for the MergeWith pagination-clear fix).
func TestList_PerQueryUnpaginatedOverridesDefault(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	// Configure a tiny default page size.
	repo.SetDefaultListQuery(r3.Query{Pagination: r3.NewPaginationSpecWithSize(1)})

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})

	// Default applies: capped at 1, but total reflects all matches.
	capped, total, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, capped, 1)
	assert.Equal(t, int64(3), total)

	// Unpaginated clears the inherited cap and returns everything.
	all, total, err := repo.List(ctx, r3.Query{Pagination: r3.Unpaginated()})
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Equal(t, int64(3), total)

	// A custom size still overrides the default.
	two, _, err := repo.List(ctx, r3.Query{Pagination: r3.NewPaginationSpec(1, 2)})
	require.NoError(t, err)
	assert.Len(t, two, 2)
}

func TestCount_EmptyCollection(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())

	n, err := repo.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestCount_ExcludesSoftDeleted(t *testing.T) {
	repo := newJSONRepo[CityWithSoftDelete, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, CityWithSoftDelete{Name: "Berlin", Country: "Germany"})
	munich, _ := repo.Create(ctx, CityWithSoftDelete{Name: "Munich", Country: "Germany"})

	require.NoError(t, repo.Delete(ctx, munich.ID))

	n, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// IncludeTrashed counts soft-deleted rows too.
	nTrashed, err := repo.Count(ctx, r3.Query{IncludeTrashed: maybe.Some(true)})
	require.NoError(t, err)
	assert.Equal(t, int64(2), nTrashed)
}
