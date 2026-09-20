package r3gorm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// paged is a minimal keyset-pageable model: a sortable PK and nothing else.
type paged struct {
	ID   int64  `r3:"id,pk" gorm:"primaryKey"`
	Name string `r3:"name"`
}

func setupPaged(t *testing.T) *r3gorm.GormCRUD[paged, int64] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&paged{}))

	repo := r3gorm.NewGormCRUD[paged, int64](db)
	ctx := context.Background()
	for _, name := range []string{"one", "two", "three", "four", "five"} {
		_, err := repo.Create(ctx, paged{Name: name})
		require.NoError(t, err)
	}
	return repo // ids 1..5
}

// The driver ignored Query.Cursor entirely, so every case here previously came
// back as the whole unpaginated table with a real total count.
func TestGormCursorPagination(t *testing.T) {
	ctx := context.Background()
	sortByID := r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("id"))}

	t.Run("forward cursor returns one keyset page", func(t *testing.T) {
		repo := setupPaged(t)
		token, err := r3.EncodeCursor(r3.CursorValues{"id": int64(2)})
		require.NoError(t, err)

		got, count, err := repo.List(ctx, r3.Query{
			Sorts:  sortByID,
			Cursor: r3.NewCursorAfter(token, 2),
		})
		require.NoError(t, err)
		require.Len(t, got, 2, "a cursor page is CursorLimit rows, not the whole table")
		assert.Equal(t, int64(3), got[0].ID)
		assert.Equal(t, int64(4), got[1].ID)
		assert.Equal(t, int64(-1), count, "keyset pagination has no total count")
	})

	t.Run("first page has no keyset predicate", func(t *testing.T) {
		repo := setupPaged(t)
		got, count, err := repo.List(ctx, r3.Query{
			Sorts:  sortByID,
			Cursor: r3.NewCursorFirst(2),
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(1), got[0].ID)
		assert.Equal(t, int64(2), got[1].ID)
		assert.Equal(t, int64(-1), count)
	})

	t.Run("backward cursor returns the rows immediately before, in order", func(t *testing.T) {
		repo := setupPaged(t)
		token, err := r3.EncodeCursor(r3.CursorValues{"id": int64(5)})
		require.NoError(t, err)

		got, count, err := repo.List(ctx, r3.Query{
			Sorts:  sortByID,
			Cursor: r3.NewCursorBefore(token, 2),
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(3), got[0].ID, "backward page scans reversed, then restores the requested order")
		assert.Equal(t, int64(4), got[1].ID)
		assert.Equal(t, int64(-1), count)
	})

	t.Run("a cursor without sorts is rejected", func(t *testing.T) {
		token, err := r3.EncodeCursor(r3.CursorValues{"id": int64(2)})
		require.NoError(t, err)

		_, _, err = setupPaged(t).List(ctx, r3.Query{Cursor: r3.NewCursorAfter(token, 2)})
		require.ErrorIs(t, err, r3.ErrCursorRequiresSort)
	})

	t.Run("offset pagination is untouched", func(t *testing.T) {
		got, count, err := setupPaged(t).List(ctx, r3.Query{
			Sorts:      sortByID,
			Pagination: r3.NewOffsetPagination(1, 2),
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(2), got[0].ID)
		assert.Equal(t, int64(5), count, "offset pagination still reports the total")
	})
}
