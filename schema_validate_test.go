package r3_test

import (
	"testing"

	"github.com/expectto/be"
	"github.com/expectto/be/be_string"

	"github.com/amberpixels/r3"
)

func TestValidateQuery_AcceptsAllowedFields(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	q := r3.NewQuery()
	q.Filters = r3.Filters{
		r3.Eq("title", "x"),
		r3.Gt("weight", 3),
		r3.And(r3.Eq("active", true), r3.Or(r3.Eq("status", "draft"), r3.Eq("slug", "a"))),
	}
	q.Sorts = r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("created_at"))}
	q.Fields = r3.Fields{r3.NewFieldSpec("title"), r3.NewFieldSpec("id")}

	be.NoError(t, s.ValidateQuery(q))
}

func TestValidateQuery_RejectsTypedErrors(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()

	cases := []struct {
		name    string
		query   r3.Query
		wantErr error
		field   string
	}{
		{
			name:    "non-filterable field",
			query:   r3.Query{Filters: r3.Filters{r3.Eq("secret_token", "x")}},
			wantErr: r3.ErrFieldNotFilterable,
			field:   "secret_token",
		},
		{
			name:    "non-filterable nested in OR",
			query:   r3.Query{Filters: r3.Filters{r3.Or(r3.Eq("title", "a"), r3.Eq("secret_token", "b"))}},
			wantErr: r3.ErrFieldNotFilterable,
			field:   "secret_token",
		},
		{
			name:    "non-sortable field",
			query:   r3.Query{Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("secret_token"))}},
			wantErr: r3.ErrFieldNotSortable,
			field:   "secret_token",
		},
		{
			name:    "non-queryable field",
			query:   r3.Query{Fields: r3.Fields{r3.NewFieldSpec("secret_token")}},
			wantErr: r3.ErrFieldNotQueryable,
			field:   "secret_token",
		},
		{
			name:    "unknown filter field",
			query:   r3.Query{Filters: r3.Filters{r3.Eq("nope", 1)}},
			wantErr: r3.ErrUnknownField,
			field:   "nope",
		},
		{
			name:    "unknown sort field",
			query:   r3.Query{Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("nope"))}},
			wantErr: r3.ErrUnknownField,
			field:   "nope",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateQuery(tc.query)
			be.ErrorIs(t, err, tc.wantErr)
			be.AssertThat(t, err.Error(), be_string.ContainingSubstring(tc.field))
		})
	}
}

func TestValidateQuery_SkipsRelationAndDottedFields(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()

	// Dotted (join/relation path) fields are validated by the engine, not here.
	q := r3.Query{
		Filters: r3.Filters{r3.Eq("city.name", "Berlin")},
		Sorts:   r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("city.name"))},
	}
	be.AssertThat(t, s.ValidateQuery(q), be.Succeed())

	// Relationship ("has") filters are resolved by the driver — skip them here.
	hasQ := r3.Query{Filters: r3.Filters{r3.Has("Children", r3.Eq("name", "x"))}}
	be.AssertThat(t, s.ValidateQuery(hasQ), be.Succeed())
}

func TestValidateQuery_ZeroSchemaValidatesNothing(t *testing.T) {
	var s r3.Schema
	be.RequireThat(t, s.IsZero(), be.True())
	q := r3.Query{Filters: r3.Filters{r3.Eq("anything", 1)}}
	be.AssertThat(t, s.ValidateQuery(q), be.Succeed())
}
