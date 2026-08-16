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

// blobbed is the shape projection exists for: a couple of scalars a listing
// renders, beside a fat column it never reads.
type blobbed struct {
	ID   int64  `r3:"id,pk" gorm:"primaryKey"`
	Name string `r3:"name"`
	Blob string `r3:"blob"`
}

func setupBlobbed(t *testing.T) *r3gorm.GormCRUD[blobbed, int64] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&blobbed{}))

	repo := r3gorm.NewGormCRUD[blobbed, int64](db)
	ctx := context.Background()
	for _, name := range []string{"first", "second"} {
		_, err := repo.Create(ctx, blobbed{Name: name, Blob: "a very large payload"})
		require.NoError(t, err)
	}
	return repo
}

func TestGormProjection(t *testing.T) {
	ctx := context.Background()

	t.Run("no projection returns every column", func(t *testing.T) {
		got, _, err := setupBlobbed(t).List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "a very large payload", got[0].Blob)
	})

	t.Run("List with ExcludeFields drops the column and keeps the rest", func(t *testing.T) {
		got, _, err := setupBlobbed(t).List(ctx, r3.Query{ExcludeFields: r3.Exclude("blob")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, row := range got {
			assert.Empty(t, row.Blob, "excluded column must not be selected")
			assert.NotEmpty(t, row.Name, "unexcluded columns are untouched")
			assert.NotZero(t, row.ID, "the PK survives every projection")
		}
	})

	t.Run("Get with ExcludeFields does the same", func(t *testing.T) {
		got, err := setupBlobbed(t).Get(ctx, 1, r3.Query{ExcludeFields: r3.Exclude("blob")})
		require.NoError(t, err)
		assert.Equal(t, int64(1), got.ID)
		assert.NotEmpty(t, got.Name)
		assert.Empty(t, got.Blob)
	})

	t.Run("Fields still keeps only what it names", func(t *testing.T) {
		got, _, err := setupBlobbed(t).List(ctx, r3.Query{Fields: r3.Include("name")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.NotEmpty(t, got[0].Name)
		assert.Empty(t, got[0].Blob)
	})

	t.Run("an unknown excluded column is a typo, not a no-op", func(t *testing.T) {
		repo := setupBlobbed(t)

		_, _, err := repo.List(ctx, r3.Query{ExcludeFields: r3.Exclude("bolb")})
		require.ErrorIs(t, err, r3.ErrUnknownField)

		_, err = repo.Get(ctx, 1, r3.Query{ExcludeFields: r3.Exclude("bolb")})
		require.ErrorIs(t, err, r3.ErrUnknownField)
	})

	t.Run("both projection forms at once is a conflict", func(t *testing.T) {
		_, _, err := setupBlobbed(t).List(ctx, r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("blob"),
		})
		assert.ErrorIs(t, err, r3.ErrProjectionConflict)
	})

	// The row keeps everything: a projection narrows the answer, never the table.
	t.Run("the stored row keeps everything", func(t *testing.T) {
		repo := setupBlobbed(t)
		_, err := repo.Get(ctx, 1, r3.Query{ExcludeFields: r3.Exclude("blob")})
		require.NoError(t, err)

		full, err := repo.Get(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "a very large payload", full.Blob)
	})
}
