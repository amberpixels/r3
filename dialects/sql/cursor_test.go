package r3sql_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/expectto/be"
	"github.com/expectto/be/be_string"
	betestify "github.com/expectto/be/x/testify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE on MatchTemplate + SQL:
// be_string.MatchTemplate turns "{{name}}" into a capture group and treats all
// other template text as LITERALS (SQL metacharacters like '(', ')', '?' need no
// escaping). The template is anchored: it must match the whole string.

func TestCursorToSQLClause_SingleColumnDesc(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"created_at": "2024-01-15T10:00:00Z"}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	betestify.Require(t, err, be.Succeed())

	// Exact assertion keeps correctness pinned.
	betestify.Assert(t, clause.Clause, be.Eq(`"created_at" < ?`))

	// Structural assertion: a quoted column, the comparison operator '<' and a '?'
	// placeholder — all literal text, matched against the whole string.
	betestify.Assert(t, clause.Clause, be_string.MatchTemplate(
		`{{col}} < ?`,
		be_string.V("col", be_string.MatchWildcard(`"*"`)),
	))

	betestify.Assert(t, clause.Args, be.Eq([]any{"2024-01-15T10:00:00Z"}))
}

func TestCursorToSQLClause_SingleColumnAsc(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))}
	values := r3.CursorValues{"name": "Alice"}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	betestify.Require(t, err, be.Succeed())

	betestify.Assert(t, clause.Clause, be.Eq(`"name" > ?`))

	// ASC forward -> '>' operator. Constrain the column to a double-quoted identifier.
	betestify.Assert(t, clause.Clause, be_string.MatchTemplate(
		`{{col}} > ?`,
		be_string.V("col", be_string.MatchWildcard(`"*"`)),
	))

	betestify.Assert(t, clause.Args, be.Eq([]any{"Alice"}))
}

func TestCursorToSQLClause_SingleColumnBackward(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"created_at": "2024-01-15T10:00:00Z"}

	clause, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorBackward)
	betestify.Require(t, err, be.Succeed())

	// Backward on DESC -> use > (flip)
	betestify.Assert(t, clause.Clause, be.Eq(`"created_at" > ?`))

	// The flip is the interesting behavior: assert the operator is '>' via template.
	betestify.Assert(t, clause.Clause, be_string.MatchTemplate(
		`"created_at" {{op}} ?`,
		be_string.V("op", be.Eq(">")),
	))
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
	betestify.Require(t, err, be.Succeed())

	// Expected: (("created_at" < ?) OR ("created_at" = ? AND "id" > ?))
	expected := `(("created_at" < ?) OR ("created_at" = ? AND "id" > ?))`
	betestify.Assert(t, clause.Clause, be.Eq(expected))

	// Structural template for the compound keyset condition — parens and question
	// marks are plain literals.
	betestify.Assert(t, clause.Clause, be_string.MatchTemplate(
		`(({{c1}} < ?) OR ({{c2}} = ? AND {{c3}} > ?))`,
		be_string.V("c1", be_string.MatchWildcard(`"*"`)),
		be_string.V("c2", be_string.MatchWildcard(`"*"`)),
		be_string.V("c3", be_string.MatchWildcard(`"*"`)),
	))

	// Looser, far more readable structural check via wildcard.
	betestify.Assert(t, clause.Clause, be_string.MatchWildcard(
		`*("created_at" < ?) OR ("created_at" = ? AND "id" > ?)*`,
	))

	betestify.Assert(t, clause.Clause, be_string.ContainingSubstring(`OR`))

	betestify.Assert(t, clause.Args, be.Eq([]any{
		"2024-01-15T10:00:00Z",
		"2024-01-15T10:00:00Z", float64(42),
	}))
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
	betestify.Require(t, err, be.Succeed())

	expected := `(("score" < ?) OR ("score" = ? AND "name" > ?) OR ("score" = ? AND "name" = ? AND "id" > ?))`
	betestify.Assert(t, clause.Clause, be.Eq(expected))

	// Full structural template across three OR-branches.
	betestify.Assert(t, clause.Clause, be_string.MatchTemplate(
		`(({{a}} < ?) OR ({{b}} = ? AND {{c}} > ?) OR `+
			`({{d}} = ? AND {{e}} = ? AND {{f}} > ?))`,
		be_string.V("a", be_string.MatchWildcard(`"*"`)),
		be_string.V("b", be_string.MatchWildcard(`"*"`)),
		be_string.V("c", be_string.MatchWildcard(`"*"`)),
		be_string.V("d", be_string.MatchWildcard(`"*"`)),
		be_string.V("e", be_string.MatchWildcard(`"*"`)),
		be_string.V("f", be_string.MatchWildcard(`"*"`)),
	))

	betestify.Assert(t, clause.Args, be.HaveLength(6))
}

func TestCursorToSQLClause_NoSorts(t *testing.T) {
	_, err := r3sql.CursorToSQLClause(r3.CursorValues{"a": 1}, nil, r3.CursorForward)
	// be has no direct ErrorIs equivalent; keep testify for the sentinel-error check.
	assert.ErrorIs(t, err, r3.ErrCursorRequiresSort)
}

func TestCursorToSQLClause_MissingValue(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"wrong_col": "val"}

	_, err := r3sql.CursorToSQLClause(values, sorts, r3.CursorForward)
	require.ErrorIs(t, err, r3.ErrInvalidCursor)

	// The error message structure is itself a "template": exercise MatchTemplate on it.
	betestify.Assert(t, err.Error(), be_string.ContainingSubstring("missing value for sort column"))
	betestify.Assert(t, err.Error(), be_string.MatchTemplate(
		`invalid cursor token: missing value for sort column {{col}}`,
		be_string.V("col", be_string.NonEmptyString()),
	))
}

func TestCursorToSQLClause_EmptyValues(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}

	clause, err := r3sql.CursorToSQLClause(r3.CursorValues{}, sorts, r3.CursorForward)
	betestify.Require(t, err, be.Succeed())
	betestify.Assert(t, clause.Clause, be_string.EmptyString())
}
