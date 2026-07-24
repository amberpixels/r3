package r3bson_test

import (
	"testing"
	"time"

	"github.com/expectto/be"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

// minuteOfDayExpr mirrors the aggregation expression the dialect builds for a
// field's minute-of-day, so the golden documents read declaratively.
func minuteOfDayExpr(field string) bson.D {
	ref := "$" + field
	return bson.D{{Key: "$add", Value: bson.A{
		bson.D{{Key: "$multiply", Value: bson.A{
			bson.D{{Key: "$hour", Value: ref}},
			60,
		}}},
		bson.D{{Key: "$minute", Value: ref}},
	}}}
}

func TestFilterToBSON_WeekdayIn(t *testing.T) {
	// Go Saturday=6, Sunday=0 -> Mongo $dayOfWeek 7 and 1.
	f := r3.WeekdayIn("started_at", time.Saturday, time.Sunday)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	expected := bson.D{{Key: "$expr", Value: bson.D{{Key: "$in", Value: bson.A{
		bson.D{{Key: "$dayOfWeek", Value: "$started_at"}},
		bson.A{7, 1},
	}}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_TimeOfDayBetween_NonWrapping(t *testing.T) {
	// mornings 05:00-12:00 -> [300, 720).
	f := r3.TimeOfDayBetween("started_at", 300, 720)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	mod := minuteOfDayExpr("started_at")
	expected := bson.D{{Key: "$expr", Value: bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "$gte", Value: bson.A{mod, 300}}},
		bson.D{{Key: "$lt", Value: bson.A{mod, 720}}},
	}}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_TimeOfDayBetween_Wrapping(t *testing.T) {
	// nights 22:00-05:00 -> lo 1320 > hi 300, OR-joined.
	f := r3.TimeOfDayBetween("started_at", 1320, 300)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	mod := minuteOfDayExpr("started_at")
	expected := bson.D{{Key: "$expr", Value: bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "$gte", Value: bson.A{mod, 1320}}},
		bson.D{{Key: "$lt", Value: bson.A{mod, 300}}},
	}}}}}
	assertBSONEqual(t, expected, doc)
}

func TestFilterToBSON_TimePattern_ComposesInOrGroup(t *testing.T) {
	// A $expr document must compose as a member of an $or group.
	f := r3.Or(
		r3.WeekdayIn("started_at", time.Sunday),
		r3.TimeOfDayBetween("started_at", 300, 720),
	)

	doc, err := r3bson.FilterToBSON(f)
	be.NoError(t, err)

	mod := minuteOfDayExpr("started_at")
	expected := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "$expr", Value: bson.D{{Key: "$in", Value: bson.A{
			bson.D{{Key: "$dayOfWeek", Value: "$started_at"}},
			bson.A{1},
		}}}}},
		bson.D{{Key: "$expr", Value: bson.D{{Key: "$and", Value: bson.A{
			bson.D{{Key: "$gte", Value: bson.A{mod, 300}}},
			bson.D{{Key: "$lt", Value: bson.A{mod, 720}}},
		}}}}},
	}}}
	assertBSONEqual(t, expected, doc)
}

func TestOperatorToBSON_TimePatternRejected(t *testing.T) {
	// The scalar operator path cannot express these; they must go via FilterToBSON.
	_, err := r3bson.OperatorToBSON(r3.OperatorWeekdayIn)
	be.Error(t, err)
	_, err = r3bson.OperatorToBSON(r3.OperatorTimeOfDayBetween)
	be.Error(t, err)
}
