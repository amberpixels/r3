package r3sql_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

// fakeJSON renders a JSON field as JX(col|a.b|n) so tests can see the hook ran.
var fakeJSON = r3sql.RenderOptions{
	JSONPathExpr: func(col string, path []string, numeric bool) (string, error) {
		mode := "t"
		if numeric {
			mode = "n"
		}
		return "JX(" + col + "|" + strings.Join(path, ".") + "|" + mode + ")", nil
	},
}

func TestJSONField_RendersThroughHook(t *testing.T) {
	breed := r3.JSONField("profile", "breed")
	age := r3.JSONField("profile", "age").AsNumber()

	clause, err := r3sql.FilterToSQLWith(r3.F(breed, "beagle"), fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, `JX("profile"|breed|t) = ?`, clause.Clause)
	assert.Empty(t, clause.Joins, "a JSON field's column is on the root table")

	clause, err = r3sql.FilterToSQLWith(r3.F(breed, nil), fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, `JX("profile"|breed|t) IS NULL`, clause.Clause)

	clause, err = r3sql.FilterToSQLWith(r3.Fop(age, r3.OperatorIn, []string{"2", "3"}), fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, `JX("profile"|age|n) IN (?, ?)`, clause.Clause)
	assert.Equal(t, []any{2.0, 3.0}, clause.Args, "numeric strings bind as numbers")

	clause, err = r3sql.FilterToSQLWith(r3.Fop(age, r3.OperatorBetween, []any{1, "4"}), fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, []any{1, 4.0}, clause.Args)

	sorts, err := r3sql.SortsToSQLWith(r3.Sorts{r3.NewSortDescSpec(age)}, fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, `JX("profile"|age|n) DESC`, sorts[0].String())

	cursor, err := r3sql.CursorToSQLClauseWith(
		r3.CursorValues{age.String(): 3.0}, r3.Sorts{r3.NewSortDescSpec(age)}, r3.CursorForward, fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, `JX("profile"|age|n) < ?`, cursor.Clause)

	groups, err := r3sql.GroupByToSQLWith(r3.Fields{breed}, fakeJSON)
	require.NoError(t, err)
	assert.Equal(t, []string{`JX("profile"|breed|t)`}, groups)
}

func TestJSONField_RejectedWithoutHook(t *testing.T) {
	_, err := r3sql.FilterToSQL(r3.F(r3.JSONField("profile", "breed"), "x"))
	require.ErrorIs(t, err, r3.ErrJSONFieldNotSupported)

	_, err = r3sql.SortToSQL(r3.NewSortAscSpec(r3.JSONField("profile", "breed")))
	require.ErrorIs(t, err, r3.ErrJSONFieldNotSupported)
}

func TestJSONField_BadSegmentRejectedBeforeHook(t *testing.T) {
	called := false
	opts := r3sql.RenderOptions{JSONPathExpr: func(string, []string, bool) (string, error) {
		called = true
		return "", nil
	}}
	for _, bad := range []*r3.FieldSpec{r3.JSONField("profile", "a b"), r3.JSONField("profile", "x;y")} {
		_, err := r3sql.FilterToSQLWith(r3.F(bad, "v"), opts)
		require.ErrorIs(t, err, r3.ErrInvalidJSONField)
	}
	assert.False(t, called)
}
