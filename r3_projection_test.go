package r3_test

import (
	"errors"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
)

// A projection is additive or subtractive, never both: MongoDB rejects the mixed
// document outright, and SQL would have to invent a precedence.
func TestQueryValidateProjection(t *testing.T) {
	t.Run("neither form is fine", func(t *testing.T) {
		be.AssertThat(t, r3.Query{}.ValidateProjection(), be.Nil())
	})

	t.Run("either form alone is fine", func(t *testing.T) {
		keep := r3.Query{Fields: r3.Include("a")}
		be.AssertThat(t, keep.ValidateProjection(), be.Nil())
		be.AssertThat(t, r3.Query{ExcludeFields: r3.Exclude("a")}.ValidateProjection(), be.Nil())
	})

	t.Run("both together conflict", func(t *testing.T) {
		q := r3.Query{Fields: r3.Include("a"), ExcludeFields: r3.Exclude("b")}
		be.AssertThat(t, errors.Is(q.ValidateProjection(), r3.ErrProjectionConflict), be.True())
	})
}

// An excluded name that matches nothing drops nothing, so backends that cannot
// run the full schema validation check the names they do know.
func TestQueryValidateProjectionAgainst(t *testing.T) {
	known := []string{"id", "name", "blob"}

	t.Run("a known excluded field passes", func(t *testing.T) {
		q := r3.Query{ExcludeFields: r3.Exclude("blob")}
		be.AssertThat(t, q.ValidateProjectionAgainst(known), be.Nil())
	})

	t.Run("an unknown excluded field is a typo, not a no-op", func(t *testing.T) {
		q := r3.Query{ExcludeFields: r3.Exclude("bolb")}
		be.AssertThat(t, errors.Is(q.ValidateProjectionAgainst(known), r3.ErrUnknownField), be.True())
	})

	t.Run("a relation path is left to the relation's own backend", func(t *testing.T) {
		q := r3.Query{ExcludeFields: r3.Exclude("owner.blob")}
		be.AssertThat(t, q.ValidateProjectionAgainst(known), be.Nil())
	})

	t.Run("Fields are not checked - an unknown name is visibly absent anyway", func(t *testing.T) {
		q := r3.Query{Fields: r3.Include("nope")}
		be.AssertThat(t, q.ValidateProjectionAgainst(known), be.Nil())
	})

	t.Run("the structural rule still applies", func(t *testing.T) {
		q := r3.Query{Fields: r3.Include("name"), ExcludeFields: r3.Exclude("blob")}
		be.AssertThat(t, errors.Is(q.ValidateProjectionAgainst(known), r3.ErrProjectionConflict), be.True())
	})
}

// ExcludeFields accumulates on merge exactly as Fields does, so a repo default
// and a per-call query both contribute.
func TestQueryMergeWith_ExcludeFieldsAccumulate(t *testing.T) {
	base := r3.Query{ExcludeFields: r3.Exclude("blob_a")}
	merged := base.MergeWith(r3.Query{ExcludeFields: r3.Exclude("blob_b")})

	be.AssertThat(t, r3.FieldsToStrings(merged.ExcludeFields), be.Eq([]string{"blob_a", "blob_b"}))
	// The receiver is never mutated.
	be.AssertThat(t, len(base.ExcludeFields), be.Eq(1))
}

// The two forms cannot coexist, so when the layers disagree the higher-precedence
// one wins. Otherwise a repo default naming Fields would make every per-call
// Exclude an unavoidable conflict.
func TestQueryMergeWith_ProjectionFormOverrides(t *testing.T) {
	t.Run("an excluding call overrides a selecting default", func(t *testing.T) {
		defaults := r3.Query{Fields: r3.Include("name")}
		merged := defaults.MergeWith(r3.Query{ExcludeFields: r3.Exclude("blob")})

		be.AssertThat(t, len(merged.Fields), be.Eq(0))
		be.AssertThat(t, r3.FieldsToStrings(merged.ExcludeFields), be.Eq([]string{"blob"}))
		be.AssertThat(t, merged.ValidateProjection(), be.Nil())
	})

	t.Run("a selecting call overrides an excluding default", func(t *testing.T) {
		defaults := r3.Query{ExcludeFields: r3.Exclude("blob")}
		merged := defaults.MergeWith(r3.Query{Fields: r3.Include("name")})

		be.AssertThat(t, len(merged.ExcludeFields), be.Eq(0))
		be.AssertThat(t, r3.FieldsToStrings(merged.Fields), be.Eq([]string{"name"}))
		be.AssertThat(t, merged.ValidateProjection(), be.Nil())
	})

	t.Run("a call naming both forms keeps its own conflict", func(t *testing.T) {
		merged := r3.Query{}.MergeWith(r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("blob"),
		})
		be.AssertThat(t, errors.Is(merged.ValidateProjection(), r3.ErrProjectionConflict), be.True())
	})

	t.Run("a call naming neither leaves the default alone", func(t *testing.T) {
		defaults := r3.Query{ExcludeFields: r3.Exclude("blob")}
		merged := defaults.MergeWith(r3.Query{Filters: r3.Filters{r3.Eq("name", "x")}})

		be.AssertThat(t, r3.FieldsToStrings(merged.ExcludeFields), be.Eq([]string{"blob"}))
	})
}

type projectionRow struct {
	ID   int64  `r3:"id,pk"`
	Name string `r3:"name"`
	Blob string `r3:"blob"`
}

// Schema validation runs the structural check even for a zero schema, and holds
// an excluded field to existence only: dropping a column that is already not
// returned is a harmless no-op, but a typo excludes nothing at all.
func TestSchemaValidateQuery_Projection(t *testing.T) {
	schema := r3.SchemaOf[projectionRow]()

	t.Run("conflict is caught without a schema", func(t *testing.T) {
		q := r3.Query{Fields: r3.Include("name"), ExcludeFields: r3.Exclude("blob")}
		be.AssertThat(t, errors.Is(r3.Schema{}.ValidateQuery(q), r3.ErrProjectionConflict), be.True())
	})

	t.Run("a known excluded field passes", func(t *testing.T) {
		be.AssertThat(t, schema.ValidateQuery(r3.Query{ExcludeFields: r3.Exclude("blob")}), be.Nil())
	})

	t.Run("an unknown excluded field is a typo, not a no-op", func(t *testing.T) {
		err := schema.ValidateQuery(r3.Query{ExcludeFields: r3.Exclude("bolb")})
		be.AssertThat(t, errors.Is(err, r3.ErrUnknownField), be.True())
	})
}
