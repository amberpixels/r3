package r3_test

import (
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
)

func TestShortFormFilterHelpers(t *testing.T) {
	tests := []struct {
		name      string
		got       *r3.FilterSpec
		wantField string
		wantOp    r3.FilterOperatorSpec
		wantVal   any
	}{
		{"Eq", r3.Eq("name", "Berlin"), "name", r3.OperatorEq, "Berlin"},
		{"Ne", r3.Ne("name", "Berlin"), "name", r3.OperatorNe, "Berlin"},
		{"Gt", r3.Gt("age", 18), "age", r3.OperatorGt, 18},
		{"Gte", r3.Gte("age", 18), "age", r3.OperatorGte, 18},
		{"Lt", r3.Lt("age", 18), "age", r3.OperatorLt, 18},
		{"Lte", r3.Lte("age", 18), "age", r3.OperatorLte, 18},
		{"In", r3.In("country", []string{"DE", "FR"}), "country", r3.OperatorIn, []string{"DE", "FR"}},
		{"NotIn", r3.NotIn("country", []string{"DE"}), "country", r3.OperatorNotIn, []string{"DE"}},
		{"Like", r3.Like("name", "B%"), "name", r3.OperatorLike, "B%"},
		{"NotLike", r3.NotLike("name", "B%"), "name", r3.OperatorNotLike, "B%"},
		{"ILike", r3.ILike("name", "b%"), "name", r3.OperatorILike, "b%"},
		{"Exists", r3.Exists("deleted_at", true), "deleted_at", r3.OperatorExists, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be.AssertThat(t, tt.got.Field.String(), be.Eq(tt.wantField))
			be.AssertThat(t, tt.got.Operator, be.Eq(tt.wantOp))
			be.AssertThat(t, tt.got.Value, be.Eq(tt.wantVal))
		})
	}
}

func TestBetweenHelper(t *testing.T) {
	f := r3.Between("age", 18, 65)
	be.AssertThat(t, f.Field.String(), be.Eq("age"))
	be.AssertThat(t, f.Operator, be.Eq(r3.OperatorBetween))
	// Between stores its bounds as a 2-element []any; assert both membership
	// and exact value so the ordering/shape is pinned down.
	be.AssertThat(t, f.Value, be.ContainElements(18, 65))
	be.AssertThat(t, f.Value, be.Eq([]any{18, 65}))
}

func TestShortFormEquivalentToVerboseForm(t *testing.T) {
	short := r3.Eq("name", "Berlin")
	verbose := r3.F(r3.NewFieldSpec("name"), "Berlin")

	be.AssertThat(t, short.Field.String(), be.Eq(verbose.Field.String()))
	be.AssertThat(t, short.Operator, be.Eq(verbose.Operator))
	be.AssertThat(t, short.Value, be.Eq(verbose.Value))
}
