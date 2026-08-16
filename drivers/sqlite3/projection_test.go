package r3sqlite3_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
)

// blobbed is the shape projection exists for: a couple of scalars a listing
// renders, beside a fat column it never reads.
type blobbed struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Blob string `db:"blob"`
}

func setupBlobbed(t *testing.T) *r3sqlite3.Sqlite3CRUD[blobbed, int64] {
	t.Helper()
	db, err := setupSQLiteDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE blobbeds (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		blob TEXT NOT NULL
	)`)
	require.NoError(t, err)

	repo := r3sqlite3.NewSqlite3CRUD[blobbed, int64](db)
	ctx := context.Background()
	for _, name := range []string{"first", "second"} {
		_, err := repo.Create(ctx, blobbed{Name: name, Blob: "a very large payload"})
		require.NoError(t, err)
	}
	return repo
}

// The raw SQL engine narrows the SELECT itself, so an excluded column never
// leaves the database.
func TestSqliteProjection(t *testing.T) {
	ctx := context.Background()
	repo := setupBlobbed(t)

	t.Run("no projection returns every column", func(t *testing.T) {
		got, _, err := repo.List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "a very large payload", got[0].Blob)
	})

	t.Run("List with ExcludeFields drops the column and keeps the rest", func(t *testing.T) {
		got, _, err := repo.List(ctx, r3.Query{ExcludeFields: r3.Exclude("blob")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, row := range got {
			assert.Empty(t, row.Blob, "excluded column must not be selected")
			assert.NotEmpty(t, row.Name, "unexcluded columns are untouched")
			assert.NotZero(t, row.ID, "the PK survives every projection")
		}
	})

	t.Run("Get with ExcludeFields does the same", func(t *testing.T) {
		got, err := repo.Get(ctx, 1, r3.Query{ExcludeFields: r3.Exclude("blob")})
		require.NoError(t, err)
		assert.Equal(t, int64(1), got.ID)
		assert.NotEmpty(t, got.Name)
		assert.Empty(t, got.Blob)
	})

	t.Run("Fields keeps only what it names, plus the PK", func(t *testing.T) {
		got, _, err := repo.List(ctx, r3.Query{Fields: r3.Include("name")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.NotEmpty(t, got[0].Name)
		assert.Empty(t, got[0].Blob)
		assert.NotZero(t, got[0].ID)
	})

	// Filtering reads a column the projection drops: the WHERE clause runs in the
	// database, so narrowing the SELECT must not change which rows come back.
	t.Run("filtering on an excluded column still works", func(t *testing.T) {
		got, _, err := repo.List(ctx, r3.Query{
			Filters:       r3.Filters{r3.Eq("blob", "a very large payload")},
			ExcludeFields: r3.Exclude("blob"),
		})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("an unknown excluded column is a typo, not a no-op", func(t *testing.T) {
		_, _, err := repo.List(ctx, r3.Query{ExcludeFields: r3.Exclude("bolb")})
		assert.ErrorIs(t, err, r3.ErrUnknownField)
	})

	t.Run("both projection forms at once is a conflict", func(t *testing.T) {
		_, _, err := repo.List(ctx, r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("blob"),
		})
		assert.ErrorIs(t, err, r3.ErrProjectionConflict)
	})

	t.Run("the stored row keeps everything", func(t *testing.T) {
		full, err := repo.Get(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "a very large payload", full.Blob)
	})
}
