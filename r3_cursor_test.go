package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	cv := r3.CursorValues{
		"created_at": "2024-01-15T10:00:00Z",
		"id":         float64(42),
	}

	token, err := r3.EncodeCursor(cv)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	decoded, err := r3.DecodeCursor(token)
	require.NoError(t, err)
	assert.Equal(t, cv, decoded)
}

func TestEncodeCursor_Empty(t *testing.T) {
	token, err := r3.EncodeCursor(nil)
	require.NoError(t, err)
	assert.Empty(t, token)

	token, err = r3.EncodeCursor(r3.CursorValues{})
	require.NoError(t, err)
	assert.Empty(t, token)
}

func TestDecodeCursor_Empty(t *testing.T) {
	cv, err := r3.DecodeCursor("")
	require.NoError(t, err)
	assert.Empty(t, cv)
}

func TestDecodeCursor_Invalid(t *testing.T) {
	_, err := r3.DecodeCursor("not-valid-base64!!!")
	require.ErrorIs(t, err, r3.ErrInvalidCursor)

	_, err = r3.DecodeCursor("bm90LWpzb24") // "not-json" in base64
	require.ErrorIs(t, err, r3.ErrInvalidCursor)
}

func TestCursorSpec_Direction(t *testing.T) {
	assert.Equal(t, r3.CursorForward, r3.NewCursorAfter("abc", 10).Direction())
	assert.Equal(t, r3.CursorBackward, r3.NewCursorBefore("abc", 10).Direction())
	assert.Equal(t, r3.CursorForward, r3.NewCursorFirst(10).Direction())

	// After takes precedence
	c := &r3.CursorSpec{After: "a", Before: "b"}
	assert.Equal(t, r3.CursorForward, c.Direction())
}

func TestCursorSpec_Token(t *testing.T) {
	assert.Equal(t, "abc", r3.NewCursorAfter("abc", 10).Token())
	assert.Equal(t, "xyz", r3.NewCursorBefore("xyz", 10).Token())
	assert.Empty(t, r3.NewCursorFirst(10).Token())
}

func TestCursorSpec_GetLimit(t *testing.T) {
	assert.Equal(t, 25, r3.NewCursorFirst(25).GetLimit())
	assert.Equal(t, r3.PageSizeDefault, r3.NewCursorFirst(0).GetLimit())
}

func TestCursorSpec_Clone(t *testing.T) {
	original := r3.NewCursorAfter("tok", 15)
	clone := original.Clone()
	assert.Equal(t, original, clone)

	clone.After = "changed"
	assert.NotEqual(t, original.After, clone.After)

	var nilSpec *r3.CursorSpec
	assert.Nil(t, nilSpec.Clone())
}

func TestCursorSpec_MergeWith(t *testing.T) {
	a := r3.NewCursorAfter("tok1", 10)
	b := r3.NewCursorAfter("tok2", 20)

	merged := a.MergeWith(b)
	assert.Equal(t, "tok2", merged.After)
	assert.Equal(t, 20, merged.Limit)

	// Nil cases
	assert.Equal(t, a, a.MergeWith(nil))

	var nilSpec *r3.CursorSpec
	assert.Equal(t, b, nilSpec.MergeWith(b))
}

func TestCursorSpec_String(t *testing.T) {
	assert.Contains(t, r3.NewCursorAfter("tok", 10).String(), "after=")
	assert.Contains(t, r3.NewCursorBefore("tok", 10).String(), "before=")
	assert.Contains(t, r3.NewCursorFirst(10).String(), "first")

	var nilSpec *r3.CursorSpec
	assert.Equal(t, "no_cursor", nilSpec.String())
}

func TestFinalizeCountCursor(t *testing.T) {
	items := []int{1, 2, 3}
	result, count := r3.FinalizeCountCursor(items)
	assert.Equal(t, items, result)
	assert.Equal(t, int64(-1), count)
}

func TestQueryCursorCloneAndMerge(t *testing.T) {
	q := r3.Query{
		Cursor: r3.NewCursorAfter("tok", 25),
	}

	clone := q.Clone()
	assert.Equal(t, q.Cursor, clone.Cursor)

	clone.Cursor.After = "changed"
	assert.NotEqual(t, q.Cursor.After, clone.Cursor.After)

	other := r3.Query{
		Cursor: r3.NewCursorAfter("new", 50),
	}
	merged := q.MergeWith(other)
	assert.Equal(t, "new", merged.Cursor.After)
	assert.Equal(t, 50, merged.Cursor.Limit)
}
