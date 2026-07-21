//nolint:testpackage // white-box goldens for the unexported $dateTrunc lowering
package enginemongo

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// White-box goldens for the $dateTrunc lowering of time-bucket group keys. The
// full pipeline runs against a real MongoDB in the driver's integration suite;
// these pin the stage shape without Docker.

func TestBucketDateTrunc_Day(t *testing.T) {
	got := bucketDateTrunc(r3.Bucket("started_at", r3.BucketDay, "day"))
	want := bson.D{{Key: dateTruncOp, Value: bson.D{
		{Key: dateTruncDateKey, Value: "$started_at"},
		{Key: dateTruncUnitKey, Value: "day"},
	}}}
	assert.Equal(t, want, got)
}

func TestBucketDateTrunc_WeekPinsMonday(t *testing.T) {
	got := bucketDateTrunc(r3.Bucket("started_at", r3.BucketWeek, "week"))
	want := bson.D{{Key: dateTruncOp, Value: bson.D{
		{Key: dateTruncDateKey, Value: "$started_at"},
		{Key: dateTruncUnitKey, Value: "week"},
		{Key: "startOfWeek", Value: "monday"},
	}}}
	assert.Equal(t, want, got, "week buckets must pin ISO-Monday and carry no timezone")
}

func TestBuildGroupAndProject_BucketKeys(t *testing.T) {
	group, project, err := buildGroupAndProject(
		[]string{"store_id"},
		r3.Buckets{r3.Bucket("started_at", r3.BucketDay, "day")},
		r3.Aggregates{r3.AggCount("n")},
	)
	require.NoError(t, err)

	// _id carries the plain key g0 and the bucket key b0.
	id := group[0].Value.(bson.D)
	assert.Equal(t, "g0", id[0].Key)
	assert.Equal(t, "$store_id", id[0].Value)
	assert.Equal(t, "b0", id[1].Key)
	assert.IsType(t, bson.D{}, id[1].Value, "bucket key lowers to a $dateTrunc doc")

	// $project flattens g0 -> store_id and b0 -> the bucket alias.
	projKeys := map[string]any{}
	for _, e := range project {
		projKeys[e.Key] = e.Value
	}
	assert.Equal(t, "$_id.g0", projKeys["store_id"])
	assert.Equal(t, "$_id.b0", projKeys["day"])
}
