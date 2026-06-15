package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A per-query Unpaginated()/NoPagination() must clear an inherited default page
// size — otherwise "give me everything" silently stays capped at the default.
func TestQueryMergeWith_UnpaginatedClearsDefault(t *testing.T) {
	base := r3.Query{Pagination: r3.NewPaginationSpecWithSize(100)}

	t.Run("Unpaginated clears the cap", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{Pagination: r3.Unpaginated()})
		assert.False(t, merged.Pagination.IsPaginated(), "default cap must be cleared")
	})

	t.Run("NoPagination clears the cap", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{Pagination: r3.NoPagination()})
		assert.False(t, merged.Pagination.IsPaginated(), "default cap must be cleared")
	})

	t.Run("custom size overrides the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{Pagination: r3.NewPaginationSpecWithSize(250)})
		assert.True(t, merged.Pagination.IsPaginated())
		assert.Equal(t, 250, merged.Pagination.GetPageSize())
	})

	t.Run("partial override keeps the inherited size", func(t *testing.T) {
		// Only the page number is set on the override; the default size survives.
		merged := base.MergeWith(r3.Query{Pagination: r3.NewPaginationSpec(3)})
		assert.True(t, merged.Pagination.IsPaginated())
		assert.Equal(t, 3, merged.Pagination.GetPageNum())
		assert.Equal(t, 100, merged.Pagination.GetPageSize())
	})

	t.Run("no override inherits the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{})
		assert.True(t, merged.Pagination.IsPaginated())
		assert.Equal(t, 100, merged.Pagination.GetPageSize())
	})
}

// A per-query sort must REPLACE the inherited default sort, not stack under it.
// Otherwise a default "created_at DESC" stays the primary key and the requested
// "price ASC" is silently demoted to a tie-breaker (the petstore sort bug).
func TestQueryMergeWith_SortOverridesDefault(t *testing.T) {
	base := r3.Query{
		Sorts: r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))},
	}

	t.Run("requested sort replaces the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{
			Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("price"))},
		})
		require.Len(t, merged.Sorts, 1, "default sort must not remain")
		assert.Equal(t, "price", merged.Sorts[0].Column.String())
		assert.Equal(t, r3.SortDirectionAsc, merged.Sorts[0].Direction)
	})

	t.Run("no requested sort inherits the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{})
		require.Len(t, merged.Sorts, 1)
		assert.Equal(t, "created_at", merged.Sorts[0].Column.String())
		assert.Equal(t, r3.SortDirectionDesc, merged.Sorts[0].Direction)
	})

	t.Run("multi-key requested sort replaces the default entirely", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{
			Sorts: r3.Sorts{
				r3.NewSortAscSpec(r3.NewFieldSpec("status")),
				r3.NewSortDescSpec(r3.NewFieldSpec("price")),
			},
		})
		require.Len(t, merged.Sorts, 2)
		assert.Equal(t, "status", merged.Sorts[0].Column.String())
		assert.Equal(t, "price", merged.Sorts[1].Column.String())
	})

	t.Run("merge does not mutate the base default", func(t *testing.T) {
		_ = base.MergeWith(r3.Query{Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("price"))}})
		require.Len(t, base.Sorts, 1)
		assert.Equal(t, "created_at", base.Sorts[0].Column.String())
	})
}

func TestDefaultsManager_UnpaginatedConfig(t *testing.T) {
	dm := r3.NewDefaultsManagerWithConfig(r3.Config{
		Defaults: r3.DefaultsConfig{Unpaginated: true},
	})

	// The default list query is unbounded...
	assert.False(t, dm.GetDefaultListQuery().Pagination.IsPaginated())
	// ...and stays unbounded when merged with an empty user query.
	assert.False(t, dm.MergeListQuery().Pagination.IsPaginated())

	// ...but an individual query can still opt into pagination.
	merged := dm.MergeListQuery(r3.Query{Pagination: r3.NewPaginationSpec(1, 25)})
	assert.True(t, merged.Pagination.IsPaginated())
	assert.Equal(t, 25, merged.Pagination.GetPageSize())
}

func TestDefaultsManager_PageSizeConfig(t *testing.T) {
	dm := r3.NewDefaultsManagerWithConfig(r3.Config{
		Defaults: r3.DefaultsConfig{PageSize: 10},
	})

	assert.Equal(t, 10, dm.GetDefaultListQuery().Pagination.GetPageSize())
}
