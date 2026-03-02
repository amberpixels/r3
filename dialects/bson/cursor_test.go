package r3bson_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCursorToBSON_SingleColumnDesc(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"created_at": "2024-01-15T10:00:00Z"}

	doc, err := r3bson.CursorToBSON(values, sorts, r3.CursorForward)
	require.NoError(t, err)

	expected := bson.D{{Key: "created_at", Value: bson.D{{Key: "$lt", Value: "2024-01-15T10:00:00Z"}}}}
	assert.Equal(t, expected, doc)
}

func TestCursorToBSON_SingleColumnAsc(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))}
	values := r3.CursorValues{"name": "Alice"}

	doc, err := r3bson.CursorToBSON(values, sorts, r3.CursorForward)
	require.NoError(t, err)

	expected := bson.D{{Key: "name", Value: bson.D{{Key: "$gt", Value: "Alice"}}}}
	assert.Equal(t, expected, doc)
}

func TestCursorToBSON_SingleColumnBackward(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"created_at": "2024-01-15T10:00:00Z"}

	doc, err := r3bson.CursorToBSON(values, sorts, r3.CursorBackward)
	require.NoError(t, err)

	// Backward on DESC -> $gt (flip)
	expected := bson.D{{Key: "created_at", Value: bson.D{{Key: "$gt", Value: "2024-01-15T10:00:00Z"}}}}
	assert.Equal(t, expected, doc)
}

func TestCursorToBSON_MultiColumn(t *testing.T) {
	sorts := r3.Sorts{
		r3.NewSortDescSpec(r3.NewFieldSpec("created_at")),
		r3.NewSortAscSpec(r3.NewFieldSpec("id")),
	}
	values := r3.CursorValues{
		"created_at": "2024-01-15T10:00:00Z",
		"id":         float64(42),
	}

	doc, err := r3bson.CursorToBSON(values, sorts, r3.CursorForward)
	require.NoError(t, err)

	expected := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "created_at", Value: bson.D{{Key: "$lt", Value: "2024-01-15T10:00:00Z"}}}},
		bson.D{{Key: "$and", Value: bson.A{
			bson.D{{Key: "created_at", Value: bson.D{{Key: "$eq", Value: "2024-01-15T10:00:00Z"}}}},
			bson.D{{Key: "id", Value: bson.D{{Key: "$gt", Value: float64(42)}}}},
		}}},
	}}}
	assert.Equal(t, expected, doc)
}

func TestCursorToBSON_NoSorts(t *testing.T) {
	_, err := r3bson.CursorToBSON(r3.CursorValues{"a": 1}, nil, r3.CursorForward)
	assert.ErrorIs(t, err, r3.ErrCursorRequiresSort)
}

func TestCursorToBSON_MissingValue(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}
	values := r3.CursorValues{"wrong_col": "val"}

	_, err := r3bson.CursorToBSON(values, sorts, r3.CursorForward)
	assert.ErrorIs(t, err, r3.ErrInvalidCursor)
}

func TestCursorToBSON_EmptyValues(t *testing.T) {
	sorts := r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))}

	doc, err := r3bson.CursorToBSON(r3.CursorValues{}, sorts, r3.CursorForward)
	require.NoError(t, err)
	assert.Equal(t, bson.D{}, doc)
}
