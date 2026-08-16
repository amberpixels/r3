package enginesql_test

import (
	"errors"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

type projectionRow struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Blob string `db:"blob"`
}

// SelectColumns is what the ORM drivers hand to Select()/Column(). Both
// projection forms have to arrive there as one explicit column list.
func TestSelectColumns(t *testing.T) {
	meta := enginesql.GetStructMeta[projectionRow]()

	t.Run("no projection leaves the ORM's own column choice alone", func(t *testing.T) {
		cols, err := meta.SelectColumns(r3.Query{})
		be.NoError(t, err)
		be.AssertThat(t, len(cols), be.Eq(0))
	})

	t.Run("Fields pass through as named", func(t *testing.T) {
		cols, err := meta.SelectColumns(r3.Query{Fields: r3.Include("name")})
		be.NoError(t, err)
		be.AssertThat(t, cols, be.Eq([]string{"name"}))
	})

	t.Run("ExcludeFields resolve to every other column, PK included", func(t *testing.T) {
		cols, err := meta.SelectColumns(r3.Query{ExcludeFields: r3.Exclude("blob")})
		be.NoError(t, err)
		be.AssertThat(t, cols, be.Eq([]string{"id", "name"}))
	})

	t.Run("the PK survives an attempt to exclude it", func(t *testing.T) {
		cols, err := meta.SelectColumns(r3.Query{ExcludeFields: r3.Exclude("id", "blob")})
		be.NoError(t, err)
		be.AssertThat(t, cols, be.Eq([]string{"id", "name"}))
	})

	t.Run("an unknown excluded column is a typo, not a no-op", func(t *testing.T) {
		_, err := meta.SelectColumns(r3.Query{ExcludeFields: r3.Exclude("bolb")})
		be.AssertThat(t, errors.Is(err, r3.ErrUnknownField), be.True())
	})

	t.Run("both forms at once conflict", func(t *testing.T) {
		_, err := meta.SelectColumns(r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("blob"),
		})
		be.AssertThat(t, errors.Is(err, r3.ErrProjectionConflict), be.True())
	})
}

// ProjectionColumns is the raw-SQL counterpart: it also hands back the struct
// field indices to scan into, and selects everything when nothing is projected.
func TestProjectionColumns(t *testing.T) {
	meta := enginesql.GetStructMeta[projectionRow]()

	t.Run("no projection selects every column", func(t *testing.T) {
		cols, idx := meta.ProjectionColumns(r3.Query{})
		be.AssertThat(t, cols, be.Eq([]string{"id", "name", "blob"}))
		be.AssertThat(t, len(idx), be.Eq(3))
	})

	t.Run("exclusion drops the column and its scan destination together", func(t *testing.T) {
		cols, idx := meta.ProjectionColumns(r3.Query{ExcludeFields: r3.Exclude("blob")})
		be.AssertThat(t, cols, be.Eq([]string{"id", "name"}))
		be.AssertThat(t, len(idx), be.Eq(len(cols)))
	})

	t.Run("selection adds the PK back", func(t *testing.T) {
		cols, idx := meta.ProjectionColumns(r3.Query{Fields: r3.Include("name")})
		be.AssertThat(t, cols, be.Eq([]string{"id", "name"}))
		be.AssertThat(t, len(idx), be.Eq(len(cols)))
	})
}
