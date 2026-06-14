package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, tt.wantField, tt.got.Field.String())
			assert.Equal(t, tt.wantOp, tt.got.Operator)
			assert.Equal(t, tt.wantVal, tt.got.Value)
		})
	}
}

func TestBetweenHelper(t *testing.T) {
	f := r3.Between("age", 18, 65)
	assert.Equal(t, "age", f.Field.String())
	assert.Equal(t, r3.OperatorBetween, f.Operator)
	assert.Equal(t, []any{18, 65}, f.Value)
}

func TestShortFormEquivalentToVerboseForm(t *testing.T) {
	short := r3.Eq("name", "Berlin")
	verbose := r3.F(r3.NewFieldSpec("name"), "Berlin")

	assert.Equal(t, verbose.Field.String(), short.Field.String())
	assert.Equal(t, verbose.Operator, short.Operator)
	assert.Equal(t, verbose.Value, short.Value)
}
