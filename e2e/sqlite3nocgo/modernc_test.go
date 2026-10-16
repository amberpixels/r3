// Package r3sqlite3nocgo_test runs drivers/sqlite3 over modernc.org/sqlite, a
// pure-Go SQLite driver, so cgo-free use stays supported. CI runs it with
// CGO_ENABLED=0 (`just test-nocgo`). It lives in the e2e module to keep modernc
// out of r3's go.mod and its consumers' go.sum.
package r3sqlite3nocgo_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite" // pure-Go SQLite driver for database/sql, registers "sqlite"
)

type Pet struct {
	ID      int64     `db:"id,pk"`
	Name    string    `db:"name"`
	Species string    `db:"species"`
	Age     int       `db:"age"`
	BornAt  time.Time `db:"born_at"`
}

func setupModerncDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	// Every in-memory connection is a separate database; pin the pool to one.
	db.SetMaxOpenConns(1)

	_, err = db.ExecContext(t.Context(), `CREATE TABLE pets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		species TEXT NOT NULL,
		age INTEGER NOT NULL,
		born_at DATETIME NOT NULL
	)`)
	require.NoError(t, err)

	return db
}

func TestSqlite3_Modernc(t *testing.T) {
	ctx := t.Context()
	repo := r3sqlite3.NewSqlite3CRUD[Pet, int64](setupModerncDB(t))

	bornAt := time.Date(2021, 3, 14, 9, 26, 53, 0, time.UTC)
	rex, err := repo.Create(ctx, Pet{Name: "Rex", Species: "dog", Age: 4, BornAt: bornAt})
	require.NoError(t, err)
	require.NotZero(t, rex.ID, "Create must return the generated ID")

	for _, p := range []Pet{
		{Name: "Tom", Species: "cat", Age: 2, BornAt: bornAt},
		{Name: "Bella", Species: "dog", Age: 1, BornAt: bornAt},
	} {
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	t.Run("Get", func(t *testing.T) {
		got, err := repo.Get(ctx, rex.ID, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, "Rex", got.Name)
		assert.Equal(t, 4, got.Age)
		assert.True(t, bornAt.Equal(got.BornAt), "born_at round trip: got %v", got.BornAt)
	})

	t.Run("List with filter", func(t *testing.T) {
		dogs, total, err := repo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Eq("species", "dog")},
			Sorts:   r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		require.Len(t, dogs, 2)
		assert.Equal(t, "Bella", dogs[0].Name)
		assert.Equal(t, "Rex", dogs[1].Name)
	})

	t.Run("Update", func(t *testing.T) {
		pet, err := repo.Get(ctx, rex.ID, r3.Query{})
		require.NoError(t, err)

		pet.Age = 5
		_, err = repo.Update(ctx, pet)
		require.NoError(t, err)

		got, err := repo.Get(ctx, rex.ID, r3.Query{})
		require.NoError(t, err)
		assert.Equal(t, 5, got.Age)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, rex.ID))

		_, err := repo.Get(ctx, rex.ID, r3.Query{})
		require.ErrorIs(t, err, r3.ErrNotFound)
	})
}
