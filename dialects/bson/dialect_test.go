package r3bson_test

import (
	"testing"

	"github.com/expectto/be"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

func TestFilterToBSON_SimpleEq(t *testing.T) {
	f := r3.F(r3.NewFieldSpec("name"), "Alice")

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	// Expected: {"name": {"$eq": "Alice"}}
	expected := bson.D{{Key: "name", Value: bson.D{{Key: "$eq", Value: "Alice"}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_NilValue(t *testing.T) {
	f := r3.F(r3.NewFieldSpec("deleted_at"), nil)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	// Expected: {"deleted_at": {"$eq": null}}
	expected := bson.D{{Key: "deleted_at", Value: bson.D{{Key: "$eq", Value: nil}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_Gt(t *testing.T) {
	f := r3.Fop(r3.NewFieldSpec("age"), r3.OperatorGt, 18)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "age", Value: bson.D{{Key: "$gt", Value: 18}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_In(t *testing.T) {
	f := r3.Fop(r3.NewFieldSpec("status"), r3.OperatorIn, []string{"active", "pending"})

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "status", Value: bson.D{{Key: "$in", Value: []string{"active", "pending"}}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_Like(t *testing.T) {
	f := r3.FLike(r3.NewFieldSpec("name"), "%alice%")

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	// LIKE "%alice%" -> regex "^.*alice.*$"
	expected := bson.D{{Key: "name", Value: bson.D{
		{Key: "$regex", Value: "^.*alice.*$"},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_ILike(t *testing.T) {
	f := r3.FILike(r3.NewFieldSpec("name"), "%alice%")

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	// ILIKE "%alice%" -> regex "^.*alice.*$" with options "i"
	expected := bson.D{{Key: "name", Value: bson.D{
		{Key: "$regex", Value: "^.*alice.*$"},
		{Key: "$options", Value: "i"},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_AndGroup(t *testing.T) {
	f := r3.And(
		r3.F(r3.NewFieldSpec("name"), "Alice"),
		r3.Fop(r3.NewFieldSpec("age"), r3.OperatorGt, 18),
	)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "name", Value: bson.D{{Key: "$eq", Value: "Alice"}}}},
		bson.D{{Key: "age", Value: bson.D{{Key: "$gt", Value: 18}}}},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_OrGroup(t *testing.T) {
	f := r3.Or(
		r3.F(r3.NewFieldSpec("status"), "active"),
		r3.F(r3.NewFieldSpec("status"), "pending"),
	)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "status", Value: bson.D{{Key: "$eq", Value: "active"}}}},
		bson.D{{Key: "status", Value: bson.D{{Key: "$eq", Value: "pending"}}}},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFiltersToBSON_MultipleFilters(t *testing.T) {
	filters := r3.Filters{
		r3.F(r3.NewFieldSpec("name"), "Alice"),
		r3.Fop(r3.NewFieldSpec("age"), r3.OperatorGt, 18),
	}

	doc, err := r3bson.FiltersToBSON(filters)
	be.NoError(t, err)

	// Multiple filters combined with $and
	expected := bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "name", Value: bson.D{{Key: "$eq", Value: "Alice"}}}},
		bson.D{{Key: "age", Value: bson.D{{Key: "$gt", Value: 18}}}},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFiltersToBSON_Empty(t *testing.T) {
	doc, err := r3bson.FiltersToBSON(nil)
	be.NoError(t, err)

	be.RequireThat(t, doc, be.Empty())
}

func TestSortsToBSON(t *testing.T) {
	sorts := r3.Sorts{
		r3.NewSortAscSpec(r3.NewFieldSpec("name")),
		r3.NewSortDescSpec(r3.NewFieldSpec("created_at")),
	}

	doc, err := r3bson.SortsToBSON(sorts)
	be.NoError(t, err)

	expected := bson.D{
		{Key: "name", Value: 1},
		{Key: "created_at", Value: -1},
	}
	assertBSONEqual(t, expected, doc)
}

func TestFieldsToBSON_Projection(t *testing.T) {
	fields := r3.Fields{
		r3.NewFieldSpec("name"),
		r3.NewFieldSpec("age"),
	}

	doc := r3bson.FieldsToBSON(fields)

	// Should include _id + requested fields
	expected := bson.D{
		{Key: "_id", Value: 1},
		{Key: "name", Value: 1},
		{Key: "age", Value: 1},
	}
	assertBSONEqual(t, expected, doc)
}

func TestFieldsToBSON_Empty(t *testing.T) {
	doc := r3bson.FieldsToBSON(nil)
	be.RequireThat(t, doc, be.Nil())
}

func TestFilterToBSON_InvalidField(t *testing.T) {
	f := r3.F(r3.NewFieldSpec("1invalid"), "foo")
	_, err := r3bson.FilterToBSON(f)
	be.Error(t, err)
}

func TestFilterToBSON_Between(t *testing.T) {
	// Between (inclusive both): age >= 18 AND age <= 65
	f := r3.Fop(r3.NewFieldSpec("age"), r3.OperatorBetween, []int{18, 65})

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "age", Value: bson.D{
		{Key: "$gte", Value: 18},
		{Key: "$lte", Value: 65},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_BetweenEx(t *testing.T) {
	// BetweenEx (exclusive both): age > 18 AND age < 65
	f := r3.Fop(r3.NewFieldSpec("age"), r3.OperatorBetweenEx, []int{18, 65})

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "age", Value: bson.D{
		{Key: "$gt", Value: 18},
		{Key: "$lt", Value: 65},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_BetweenExInc(t *testing.T) {
	// BetweenExInc (excl low, incl high): age > 18 AND age <= 65
	f := r3.Fop(r3.NewFieldSpec("age"), r3.OperatorBetweenExInc, []int{18, 65})

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "age", Value: bson.D{
		{Key: "$gt", Value: 18},
		{Key: "$lte", Value: 65},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_BetweenIncEx(t *testing.T) {
	// BetweenIncEx (incl low, excl high): age >= 18 AND age < 65
	f := r3.Fop(r3.NewFieldSpec("age"), r3.OperatorBetweenIncEx, []int{18, 65})

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "age", Value: bson.D{
		{Key: "$gte", Value: 18},
		{Key: "$lt", Value: 65},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_BetweenInvalidValue(t *testing.T) {
	// Non-slice value should fail
	f := r3.Fop(r3.NewFieldSpec("age"), r3.OperatorBetween, 18)
	_, err := r3bson.FilterToBSON(f)
	be.Error(t, err)

	// Wrong number of elements
	f = r3.Fop(r3.NewFieldSpec("age"), r3.OperatorBetween, []int{18})
	_, err = r3bson.FilterToBSON(f)
	be.Error(t, err)
}

// assertBSONEqual compares two bson.D documents by marshalling them to bytes.
func assertBSONEqual(t *testing.T, expected, actual bson.D) {
	t.Helper()

	expBytes, err := bson.Marshal(expected)
	be.NoError(t, err)
	actBytes, err := bson.Marshal(actual)
	be.NoError(t, err)

	be.AssertThat(t, string(actBytes), be.Eq(string(expBytes)))
}
