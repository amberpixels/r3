package r3_test

import (
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
)

// A per-query Unpaginated()/NoPagination() must clear an inherited default page
// size — otherwise "give me everything" silently stays capped at the default.
func TestQueryMergeWith_UnpaginatedClearsDefault(t *testing.T) {
	base := r3.Query{Pagination: r3.NewPaginationSpecWithSize(100)}

	t.Run("Unpaginated clears the cap", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{Pagination: r3.Unpaginated()})
		be.AssertThat(t, merged.Pagination.IsPaginated(), be.False(), "default cap must be cleared")
	})

	t.Run("NoPagination clears the cap", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{Pagination: r3.NoPagination()})
		be.AssertThat(t, merged.Pagination.IsPaginated(), be.False(), "default cap must be cleared")
	})

	t.Run("custom size overrides the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{Pagination: r3.NewPaginationSpecWithSize(250)})
		be.AssertThat(t, merged.Pagination.IsPaginated(), be.True())
		be.AssertThat(t, merged.Pagination.GetPageSize(), be.Eq(250))
	})

	t.Run("partial override keeps the inherited size", func(t *testing.T) {
		// Only the page number is set on the override; the default size survives.
		merged := base.MergeWith(r3.Query{Pagination: r3.NewPaginationSpec(3)})
		be.AssertThat(t, merged.Pagination.IsPaginated(), be.True())
		be.AssertThat(t, merged.Pagination.GetPageNum(), be.Eq(3))
		be.AssertThat(t, merged.Pagination.GetPageSize(), be.Eq(100))
	})

	t.Run("no override inherits the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{})
		be.AssertThat(t, merged.Pagination.IsPaginated(), be.True())
		be.AssertThat(t, merged.Pagination.GetPageSize(), be.Eq(100))
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
		be.RequireThat(t, merged.Sorts, be.HaveLength(1), "default sort must not remain")
		be.AssertThat(t, merged.Sorts[0].Column.String(), be.Eq("price"))
		be.AssertThat(t, merged.Sorts[0].Direction, be.Eq(r3.SortDirectionAsc))
	})

	t.Run("no requested sort inherits the default", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{})
		be.RequireThat(t, merged.Sorts, be.HaveLength(1))
		be.AssertThat(t, merged.Sorts[0].Column.String(), be.Eq("created_at"))
		be.AssertThat(t, merged.Sorts[0].Direction, be.Eq(r3.SortDirectionDesc))
	})

	t.Run("multi-key requested sort replaces the default entirely", func(t *testing.T) {
		merged := base.MergeWith(r3.Query{
			Sorts: r3.Sorts{
				r3.NewSortAscSpec(r3.NewFieldSpec("status")),
				r3.NewSortDescSpec(r3.NewFieldSpec("price")),
			},
		})
		be.RequireThat(t, merged.Sorts, be.HaveLength(2))
		be.AssertThat(t, merged.Sorts[0].Column.String(), be.Eq("status"))
		be.AssertThat(t, merged.Sorts[1].Column.String(), be.Eq("price"))
	})

	t.Run("merge does not mutate the base default", func(t *testing.T) {
		_ = base.MergeWith(r3.Query{Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("price"))}})
		be.RequireThat(t, base.Sorts, be.HaveLength(1))
		be.AssertThat(t, base.Sorts[0].Column.String(), be.Eq("created_at"))
	})
}

func TestDefaultsManager_UnpaginatedConfig(t *testing.T) {
	dm := r3.NewDefaultsManagerWithConfig(r3.Config{
		Defaults: r3.DefaultsConfig{Unpaginated: true},
	})

	// The default list query is unbounded...
	be.AssertThat(t, dm.GetDefaultListQuery().Pagination.IsPaginated(), be.False())
	// ...and stays unbounded when merged with an empty user query.
	be.AssertThat(t, dm.MergeListQuery().Pagination.IsPaginated(), be.False())

	// ...but an individual query can still opt into pagination.
	merged := dm.MergeListQuery(r3.Query{Pagination: r3.NewPaginationSpec(1, 25)})
	be.AssertThat(t, merged.Pagination.IsPaginated(), be.True())
	be.AssertThat(t, merged.Pagination.GetPageSize(), be.Eq(25))
}

func TestDefaultsManager_PageSizeConfig(t *testing.T) {
	dm := r3.NewDefaultsManagerWithConfig(r3.Config{
		Defaults: r3.DefaultsConfig{PageSize: 10},
	})

	be.AssertThat(t, dm.GetDefaultListQuery().Pagination.GetPageSize(), be.Eq(10))
}
