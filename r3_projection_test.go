package r3_test

import (
	"errors"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

// A projection is additive or subtractive, never both: MongoDB rejects the mixed
// document outright, and SQL would have to invent a precedence.
func TestQueryValidateProjection(t *testing.T) {
	t.Run("neither form is fine", func(t *testing.T) {
		be.AssertThat(t, r3.Query{}.ValidateProjection(), be.Nil())
	})

	t.Run("either form alone is fine", func(t *testing.T) {
		be.AssertThat(t, r3.Query{Fields: r3.Exclude("a")}.ValidateProjection(), be.Nil())
		be.AssertThat(t, r3.Query{ExcludeFields: r3.Exclude("a")}.ValidateProjection(), be.Nil())
	})

	t.Run("both together conflict", func(t *testing.T) {
		q := r3.Query{Fields: r3.Fields{r3.NewFieldSpec("a")}, ExcludeFields: r3.Exclude("b")}
		be.AssertThat(t, errors.Is(q.ValidateProjection(), r3.ErrProjectionConflict), be.True())
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

// The exclusion projection must never drop _id: an entity handed back without
// its identity cannot be patched, deleted, or linked. Mirrors FieldsToBSON
// always including it.
func TestExcludeFieldsToBSON(t *testing.T) {
	t.Run("empty yields no projection", func(t *testing.T) {
		be.AssertThat(t, r3bson.ExcludeFieldsToBSON(nil), be.Nil())
	})

	t.Run("each field is set to 0", func(t *testing.T) {
		got := r3bson.ExcludeFieldsToBSON(r3.Exclude("streams_json", "garmin_json"))
		be.AssertThat(t, len(got), be.Eq(2))
		be.AssertThat(t, got[0].Key, be.Eq("streams_json"))
		be.AssertThat(t, got[0].Value, be.Eq(0))
		be.AssertThat(t, got[1].Key, be.Eq("garmin_json"))
	})

	t.Run("_id is never excluded", func(t *testing.T) {
		got := r3bson.ExcludeFieldsToBSON(r3.Exclude("_id", "blob"))
		be.AssertThat(t, len(got), be.Eq(1))
		be.AssertThat(t, got[0].Key, be.Eq("blob"))
	})

	t.Run("excluding only _id yields no projection at all", func(t *testing.T) {
		be.AssertThat(t, r3bson.ExcludeFieldsToBSON(r3.Exclude("_id")), be.Nil())
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
		q := r3.Query{Fields: r3.Fields{r3.NewFieldSpec("name")}, ExcludeFields: r3.Exclude("blob")}
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
