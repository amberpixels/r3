package r3sqlite3_test

import (
	"database/sql"
	"testing"

	"github.com/amberpixels/r3"
	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// offsetRow is a minimal numbered row for exercising raw-offset pagination end
// to end. Table name derives to "offset_rows".
type offsetRow struct {
	ID int64 `db:"id,pk"`
	N  int   `db:"n"`
}

// TestOffsetPagination_EndToEnd proves that NewOffsetPagination(offset, limit)
// reaches the driver verbatim: with 200 rows, a misaligned offset (100, not a
// multiple of the limit 30) returns exactly the 101st-130th rows - the window a
// page-number API cannot express (see GitHub issue #1).
func TestOffsetPagination_EndToEnd(t *testing.T) {
	ctx := t.Context()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	// :memory: is per-connection; pin to one so seed + reads share a DB.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE offset_rows (id INTEGER PRIMARY KEY, n INTEGER NOT NULL)`)
	require.NoError(t, err)

	// Seed ids 1..200 with n == id, so sorting by id gives a known ordering.
	stmt, err := db.Prepare(`INSERT INTO offset_rows (id, n) VALUES (?, ?)`)
	require.NoError(t, err)
	defer func() { _ = stmt.Close() }()
	for i := 1; i <= 200; i++ {
		_, err = stmt.Exec(i, i)
		require.NoError(t, err)
	}

	repo := r3sqlite3.NewSqlite3CRUD[offsetRow, int64](db)

	q := r3.Query{
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("id"))},
		Pagination: r3.NewOffsetPagination(100, 30),
	}
	rows, total, err := repo.List(ctx, q)
	require.NoError(t, err)

	// total reflects the full match count, not the page size.
	assert.Equal(t, int64(200), total)

	// Exactly rows at 0-based offsets 100..129 -> ids/n 101..130.
	require.Len(t, rows, 30)
	got := make([]int, len(rows))
	for i, r := range rows {
		got[i] = r.N
	}
	want := make([]int, 30)
	for i := range want {
		want[i] = 101 + i
	}
	assert.Equal(t, want, got)
}
