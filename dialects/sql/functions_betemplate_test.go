package r3sql_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/expectto/be"
	"github.com/expectto/be/be_string"
	betestify "github.com/expectto/be/x/testify"
)

// These tests dogfood be_string.MatchTemplate (and friends) against the SQL produced
// by FilterToSQL / FiltersToSQL. They sit alongside the exact-string testify checks in
// functions_test.go and assert the *structure* of the SQL more expressively.
//
// Reminder: MatchTemplate compiles the template as an unanchored regex with
// "{{x}}" -> "(?P<x>.+)". Literal '(', ')', '?', '|' MUST be backslash-escaped, and
// you must add ^...$ yourself to anchor.

func TestFilterToSQL_Template_SimpleComparison(t *testing.T) {
	clauses, err := r3sql.FiltersToSQL(r3.Filters{
		{Field: r3.NewFieldSpec("id"), Operator: r3.OperatorEq, Value: 123},
	})
	betestify.Require(t, err, be.Succeed())
	betestify.Require(t, clauses, be.HaveLength(1))

	got := clauses[0].Clause
	betestify.Assert(t, got, be.Eq(`"id" = ?`))

	// Structural: <quoted-col> <op> <placeholder>. Constrain the operator placeholder
	// to a known comparison op, the column to a double-quoted identifier.
	betestify.Assert(t, got, be_string.MatchTemplate(
		`^{{col}} {{op}} \?$`,
		be_string.V("col", be_string.MatchWildcard(`"*"`)),
		be_string.V("op", be.Eq("=")),
	))
}

func TestFilterToSQL_Template_AllComparisonOperators(t *testing.T) {
	cases := []struct {
		op   r3.FilterOperatorSpec
		want string // expected operator token in the SQL
	}{
		{r3.OperatorEq, "="},
		{r3.OperatorNe, "!="},
		{r3.OperatorGt, ">"},
		{r3.OperatorGte, ">="},
		{r3.OperatorLt, "<"},
		{r3.OperatorLte, "<="},
		{r3.OperatorLike, "LIKE"},
		{r3.OperatorNotLike, "NOT LIKE"},
		{r3.OperatorILike, "ILIKE"},
	}

	for _, tc := range cases {
		clauses, err := r3sql.FiltersToSQL(r3.Filters{
			{Field: r3.NewFieldSpec("col"), Operator: tc.op, Value: "v"},
		})
		betestify.Require(t, err, be.Succeed())

		got := clauses[0].Clause
		// One template, the operator captured and constrained per-case. This is exactly
		// the kind of thing MatchTemplate is meant to express: "shape is col <op> ?,
		// and the op is THIS".
		betestify.Assert(t, got, be_string.MatchTemplate(
			`^"col" {{op}} \?$`,
			be_string.V("op", be.Eq(tc.want)),
		), "operator %s", tc.op)
	}
}

func TestFilterToSQL_Template_InList(t *testing.T) {
	clauses, err := r3sql.FiltersToSQL(r3.Filters{
		{Field: r3.NewFieldSpec("status"), Operator: r3.OperatorIn, Value: []string{"active", "pending", "archived"}},
	})
	betestify.Require(t, err, be.Succeed())

	got := clauses[0].Clause
	betestify.Assert(t, got, be.Eq(`"status" IN (?, ?, ?)`))

	// Capture the whole placeholder list as one group and assert its shape with a
	// nested wildcard. Parens escaped; the comma-separated '?'s become {{ph}}.
	betestify.Assert(t, got, be_string.MatchTemplate(
		`^{{col}} IN \({{ph}}\)$`,
		be_string.V("col", be_string.MatchWildcard(`"*"`)),
		be_string.V("ph", be_string.MatchWildcard(`?*?`)), // at least two placeholders
	))

	// Wildcard is the most readable way to assert a variable-length IN list.
	betestify.Assert(t, got, be_string.MatchWildcard(`"status" IN (?*?)`))
}

func TestFilterToSQL_Template_Between(t *testing.T) {
	clauses, err := r3sql.FiltersToSQL(r3.Filters{
		{Field: r3.NewFieldSpec("age"), Operator: r3.OperatorBetween, Value: []any{18, 65}},
	})
	betestify.Require(t, err, be.Succeed())

	got := clauses[0].Clause
	betestify.Assert(t, got, be.Eq(`("age" >= ? AND "age" <= ?)`))

	// Compound BETWEEN expansion: (col op1 ? AND col op2 ?). Both ops captured.
	betestify.Assert(t, got, be_string.MatchTemplate(
		`^\({{col}} {{lo}} \? AND {{col2}} {{hi}} \?\)$`,
		be_string.V("col", be.Eq(`"age"`)),
		be_string.V("col2", be.Eq(`"age"`)),
		be_string.V("lo", be.Eq(">=")),
		be_string.V("hi", be.Eq("<=")),
	))
}

func TestFilterToSQL_Template_NestedAndOr(t *testing.T) {
	clauses, err := r3sql.FiltersToSQL(r3.Filters{
		{
			And: r3.Filters{
				{Field: r3.NewFieldSpec("category"), Operator: r3.OperatorEq, Value: "electronics"},
				{
					Or: r3.Filters{
						{Field: r3.NewFieldSpec("price"), Operator: r3.OperatorLt, Value: 100},
						{Field: r3.NewFieldSpec("on_sale"), Operator: r3.OperatorEq, Value: true},
					},
				},
			},
		},
	})
	betestify.Require(t, err, be.Succeed())

	got := clauses[0].Clause
	betestify.Assert(t, got, be.Eq(`("category" = ? AND ("price" < ? OR "on_sale" = ?))`))

	// Full structural template of the nested group. Note the heavy escaping cost:
	// 4 parens + 3 question marks all need a backslash.
	betestify.Assert(t, got, be_string.MatchTemplate(
		`^\({{a}} = \? AND \({{b}} < \? OR {{c}} = \?\)\)$`,
		be_string.V("a", be_string.MatchWildcard(`"*"`)),
		be_string.V("b", be_string.MatchWildcard(`"*"`)),
		be_string.V("c", be_string.MatchWildcard(`"*"`)),
	))

	// More readable structural checks for the same thing.
	betestify.Assert(t, got, be_string.MatchWildcard(`(* AND (* OR *))`))
	betestify.Assert(t, got, be_string.ContainingSubstring(" AND "))
	betestify.Assert(t, got, be_string.ContainingSubstring(" OR "))
}

func TestFilterToSQL_Template_IsNull(t *testing.T) {
	clauses, err := r3sql.FiltersToSQL(r3.Filters{
		{Field: r3.NewFieldSpec("deleted_at"), Operator: r3.OperatorEq, Value: nil},
	})
	betestify.Require(t, err, be.Succeed())

	got := clauses[0].Clause
	betestify.Assert(t, got, be.Eq(`"deleted_at" IS NULL`))

	// No placeholder here: the template just constrains the column shape.
	betestify.Assert(t, got, be_string.MatchTemplate(
		`^{{col}} IS NULL$`,
		be_string.V("col", be_string.MatchWildcard(`"*"`)),
	))
}

func TestFilterToSQL_Template_QualifiedColumnWithJoin(t *testing.T) {
	clauses, err := r3sql.FiltersToSQL(r3.Filters{
		{Field: r3.NewFieldSpec("user.name"), Operator: r3.OperatorEq, Value: "John"},
	})
	betestify.Require(t, err, be.Succeed())

	got := clauses[0].Clause
	betestify.Assert(t, got, be.Eq(`"user"."name" = ?`))

	// Qualified column "table"."col": '.' is a regex metachar, so it is escaped between
	// the two placeholders.
	betestify.Assert(t, got, be_string.MatchTemplate(
		`^{{tbl}}\.{{col}} = \?$`,
		be_string.V("tbl", be_string.MatchWildcard(`"*"`)),
		be_string.V("col", be_string.MatchWildcard(`"*"`)),
	))

	betestify.Assert(t, clauses[0].Joins, be.Eq([]r3sql.SQLColumn{r3sql.SQLColumn(`"user"`)}))
}
