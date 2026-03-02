package r3sql_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorToSQLClause_SingleColumnDesc(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"created_at": "2024-01-15T10:00:00Z"}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	require.NoError(t, err)
	assert.Equal(t, `"created_at" < ?`, clause.Clause)
	assert.Equal(t, []any{"2024-01-15T10:00:00Z"}, clause.Args)
}

func TestCursorToSQLClause_SingleColumnAsc(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))}
	values := r3.CursorValues{"name": "Alice"}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	require.NoError(t, err)
	assert.Equal(t, `"name" > ?`, clause.Clause)
	assert.Equal(t, []any{"Alice"}, clause.Args)
}

func TestCursorToSQLClause_SingleColumnBackward(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"created_at": "2024-01-15T10:00:00Z"}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorBackward)
	require.NoError(t, err)
	// Backward on DESC -> use > (flip)
	assert.Equal(t, `"created_at" > ?`, clause.Clause)
}

func TestCursorToSQLClause_MultiColumn(t *testing.T) {
	sorts := r3.Sorts{
		r3.NewSortDescSpec(r3.NewFieldSpec("created_at")),
		r3.NewSortAscSpec(r3.NewFieldSpec("id")),
	}
	values := r3.CursorValues{
		"created_at": "2024-01-15T10:00:00Z",
		"id":         float64(42),
	}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	require.NoError(t, err)

	// Expected: ("created_at" < ?) OR ("created_at" = ? AND "id" > ?)
	expected := `(("created_at" < ?) OR ("created_at" = ? AND "id" > ?))`
	assert.Equal(t, expected, clause.Clause)
	assert.Equal(t, []any{
		"2024-01-15T10:00:00Z",
		"2024-01-15T10:00:00Z", float64(42),
	}, clause.Args)
}

func TestCursorToSQLClause_ThreeColumns(t *testing.T) {
	sorts := r3.Sorts{
		r3.NewSortDescSpec(r3.NewFieldSpec("score")),
		r3.NewSortAscSpec(r3.NewFieldSpec("name")),
		r3.NewSortAscSpec(r3.NewFieldSpec("id")),
	}
	values := r3.CursorValues{
		"score": float64(100),
		"name":  "Alice",
		"id":    float64(5),
	}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	require.NoError(t, err)

	expected := `(("score" < ?) OR ("score" = ? AND "name" > ?) OR ("score" = ? AND "name" = ? AND "id" > ?))`
	assert.Equal(t, expected, clause.Clause)
	assert.Len(t, clause.Args, 6)
}

func TestCursorToSQLClause_NoSorts(t *testing.T) {
	_, err := r3sql.CursorToSQLClause(r3.CursorValues{"a": 1}, nil, r3.CursorForward)
	assert.ErrorIs(t, err, r3.ErrCursorRequiresSort)
}

func TestCursorToSQLClause_MissingValue(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"wrong_col": "val"}

	_, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	assert.ErrorIs(t, err, r3.ErrInvalidCursor)
}

func TestCursorToSQLClause_EmptyValues(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}

	clause, err := r3sql.CursorToSQLClause(r3.CursorValues{}, sorts, r3.CursorForward)
	require.NoError(t, err)
	assert.Empty(t, clause.Clause)
}
