package r3bson_test

import (
	"testing"

	"github.com/expectto/be"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

func TestJSONField_BSONPath(t *testing.T) {
	doc, err := r3bson.FilterToBSON(r3.F(r3.JSONField("profile", "vaccinations", "count").AsNumber(), "3"))
	be.NoError(t, err)
	be.AssertThat(
		t,
		doc,
		be.Eq(bson.D{{Key: "profile.val.vaccinations.count", Value: bson.D{{Key: "$eq", Value: 3.0}}}}),
	)

	sort, err := r3bson.SortToBSON(r3.NewSortAscSpec(r3.JSONField("profile", "breed")))
	be.NoError(t, err)
	be.AssertThat(t, sort, be.Eq(bson.E{Key: "profile.val.breed", Value: 1}))

	_, err = r3bson.FilterToBSON(r3.F(r3.JSONField("profile", "a b"), "x"))
	be.ErrorIs(t, err, r3.ErrInvalidJSONField)
}
