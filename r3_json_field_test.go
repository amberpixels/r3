package r3_test

import (
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
)

type petProfile struct {
	Breed string `json:"breed"`
}

type jsonPet struct {
	ID      int64                         `r3:"id,pk"`
	Name    string                        `r3:"name"`
	Profile r3.JSONColumn[petProfile]     `r3:"profile"`
	Notes   r3.JSONColumn[map[string]any] `r3:"notes,no-output"`
}

func TestJSONField_RoundTrip(t *testing.T) {
	f := r3.JSONField("profile", "vaccinations", "count")
	be.AssertThat(t, f.String(), be.Eq("profile->vaccinations->count"))
	be.AssertThat(t, f.IsJSONField(), be.True())

	col, path, numeric, ok := f.JSONPath()
	be.AssertThat(t, ok, be.True())
	be.AssertThat(t, col, be.Eq("profile"))
	be.AssertThat(t, path, be.Eq([]string{"vaccinations", "count"}))
	be.AssertThat(t, numeric, be.False())

	n := f.AsNumber()
	be.AssertThat(t, n.String(), be.Eq("profile->vaccinations->count~number"))
	_, path, numeric, _ = n.JSONPath()
	be.AssertThat(t, path, be.Eq([]string{"vaccinations", "count"}))
	be.AssertThat(t, numeric, be.True())
	be.AssertThat(t, n.AsNumber().String(), be.Eq(n.String()))

	_, _, _, ok = r3.NewFieldSpec("owner.name").JSONPath()
	be.AssertThat(t, ok, be.False())
}

func TestValidateJSONField(t *testing.T) {
	for _, ok := range []string{"profile->breed", "profile->a->b_2", "profile->n~number"} {
		be.NoError(t, r3.ValidateJSONField(ok))
		be.NoError(t, r3.ValidateFieldName(ok))
	}
	for _, bad := range []string{
		"profile->a b", "profile->x;y", "profile->x'y", "profile->", "profile->0",
		"profile->x~text", "owner.profile->x", "->x", "profile->x->",
	} {
		be.ErrorIs(t, r3.ValidateJSONField(bad), r3.ErrInvalidJSONField)
		be.ErrorIs(t, r3.ValidateJSONField(bad), r3.ErrInvalidIdentifier)
	}
	// The strict identifier check never accepts the JSON form.
	be.ErrorIs(t, r3.ValidateIdentifier("profile->breed"), r3.ErrInvalidIdentifier)
}

func TestValidateJSONFieldValue(t *testing.T) {
	text := r3.JSONField("profile", "breed")
	num := r3.JSONField("profile", "age").AsNumber()

	be.NoError(t, r3.ValidateJSONFieldValue(text, r3.OperatorEq, "beagle"))
	be.NoError(t, r3.ValidateJSONFieldValue(text, r3.OperatorEq, nil))
	be.NoError(t, r3.ValidateJSONFieldValue(num, r3.OperatorGt, 3))
	be.NoError(t, r3.ValidateJSONFieldValue(num, r3.OperatorGt, "3.5"))
	be.NoError(t, r3.ValidateJSONFieldValue(num, r3.OperatorIn, []int{1, 2}))
	be.NoError(t, r3.ValidateJSONFieldValue(num, r3.OperatorExists, true))

	be.ErrorIs(t, r3.ValidateJSONFieldValue(text, r3.OperatorGt, 3), r3.ErrJSONFieldValueType)
	be.ErrorIs(t, r3.ValidateJSONFieldValue(text, r3.OperatorIn, []int64{1}), r3.ErrJSONFieldValueType)
	be.ErrorIs(t, r3.ValidateJSONFieldValue(num, r3.OperatorEq, "old"), r3.ErrJSONFieldValueType)
	be.ErrorIs(t, r3.ValidateJSONFieldValue(num, r3.OperatorEq, true), r3.ErrJSONFieldValueType)
}

func TestJSONFieldNumericValue(t *testing.T) {
	be.AssertThat(t, r3.JSONFieldNumericValue("2.5"), be.Eq(any(2.5)))
	be.AssertThat(t, r3.JSONFieldNumericValue(3), be.Eq(any(3)))
	be.AssertThat(t, r3.JSONFieldNumericValue([]string{"1", "x"}), be.Eq(any([]any{1.0, "x"})))
}

func TestValidateQuery_JSONField(t *testing.T) {
	s := r3.SchemaOf[jsonPet]()

	ok := r3.Query{
		Filters: r3.Filters{r3.F(r3.JSONField("profile", "breed"), "beagle")},
		Sorts:   r3.Sorts{r3.NewSortDescSpec(r3.JSONField("profile", "age").AsNumber())},
	}
	be.NoError(t, s.ValidateQuery(ok))
	be.NoError(t, s.ValidateAggregateQuery(r3.Query{
		GroupBy:    r3.Fields{r3.JSONField("profile", "breed")},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	}))

	cases := []struct {
		name    string
		field   *r3.FieldSpec
		value   any
		wantErr error
	}{
		{"plain column", r3.JSONField("name", "x"), "a", r3.ErrJSONFieldNotJSONColumn},
		{"unknown column", r3.JSONField("missing", "x"), "a", r3.ErrUnknownField},
		{"no-output column", r3.JSONField("notes", "x"), "a", r3.ErrFieldNotQueryable},
		{"bad segment", r3.JSONField("profile", "x;y"), "a", r3.ErrInvalidJSONField},
		{"number on text key", r3.JSONField("profile", "age"), 3, r3.ErrJSONFieldValueType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateQuery(r3.Query{Filters: r3.Filters{r3.F(tc.field, tc.value)}})
			be.ErrorIs(t, err, tc.wantErr)
		})
	}

	// A JSON field cannot be projected.
	be.ErrorIs(t, s.ValidateQuery(r3.Query{Fields: r3.Fields{r3.JSONField("profile", "breed")}}),
		r3.ErrInvalidJSONField)

	// The value-type check is structural, so it also runs without a schema.
	be.ErrorIs(t, r3.Schema{}.ValidateQuery(r3.Query{
		Filters: r3.Filters{r3.Or(r3.F(r3.JSONField("profile", "age"), 3))},
	}), r3.ErrJSONFieldValueType)
}
