package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/expectto/be"
	"github.com/expectto/be/be_string"
	betestify "github.com/expectto/be/x/testify"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	cv := r3.CursorValues{
		"created_at": "2024-01-15T10:00:00Z",
		"id":         float64(42),
	}

	token, err := r3.EncodeCursor(cv)
	betestify.Require(t, err, be.Succeed())
	betestify.Assert(t, token, be_string.NonEmptyString())

	decoded, err := r3.DecodeCursor(token)
	betestify.Require(t, err, be.Succeed())
	betestify.Assert(t, decoded, be.Eq(cv))
}

func TestEncodeCursor_Empty(t *testing.T) {
	token, err := r3.EncodeCursor(nil)
	betestify.Require(t, err, be.Succeed())
	betestify.Assert(t, token, be_string.EmptyString())

	token, err = r3.EncodeCursor(r3.CursorValues{})
	betestify.Require(t, err, be.Succeed())
	betestify.Assert(t, token, be_string.EmptyString())
}

func TestDecodeCursor_Empty(t *testing.T) {
	cv, err := r3.DecodeCursor("")
	betestify.Require(t, err, be.Succeed())
	betestify.Assert(t, cv, be.HaveLength(0))
}

func TestDecodeCursor_Invalid(t *testing.T) {
	_, err := r3.DecodeCursor("not-valid-base64!!!")
	betestify.Require(t, err, be.MatchError(r3.ErrInvalidCursor))

	_, err = r3.DecodeCursor("bm90LWpzb24") // "not-json" in base64
	betestify.Require(t, err, be.MatchError(r3.ErrInvalidCursor))
}

func TestCursorSpec_Direction(t *testing.T) {
	betestify.Assert(t, r3.NewCursorAfter("abc", 10).Direction(), be.Eq(r3.CursorForward))
	betestify.Assert(t, r3.NewCursorBefore("abc", 10).Direction(), be.Eq(r3.CursorBackward))
	betestify.Assert(t, r3.NewCursorFirst(10).Direction(), be.Eq(r3.CursorForward))

	// After takes precedence
	c := &r3.CursorSpec{After: "a", Before: "b"}
	betestify.Assert(t, c.Direction(), be.Eq(r3.CursorForward))
}

func TestCursorSpec_Token(t *testing.T) {
	betestify.Assert(t, r3.NewCursorAfter("abc", 10).Token(), be.Eq("abc"))
	betestify.Assert(t, r3.NewCursorBefore("xyz", 10).Token(), be.Eq("xyz"))
	betestify.Assert(t, r3.NewCursorFirst(10).Token(), be_string.EmptyString())
}

func TestCursorSpec_GetLimit(t *testing.T) {
	betestify.Assert(t, r3.NewCursorFirst(25).GetLimit(), be.Eq(25))
	betestify.Assert(t, r3.NewCursorFirst(0).GetLimit(), be.Eq(r3.PageSizeDefault))
}

func TestCursorSpec_Clone(t *testing.T) {
	original := r3.NewCursorAfter("tok", 15)
	clone := original.Clone()
	betestify.Assert(t, clone, be.Eq(original))

	clone.After = "changed"
	betestify.Assert(t, clone.After, be.Not(be.Eq(original.After)))

	var nilSpec *r3.CursorSpec
	betestify.Assert(t, nilSpec.Clone(), be.Nil())
}

func TestCursorSpec_MergeWith(t *testing.T) {
	a := r3.NewCursorAfter("tok1", 10)
	b := r3.NewCursorAfter("tok2", 20)

	merged := a.MergeWith(b)
	betestify.Assert(t, merged.After, be.Eq("tok2"))
	betestify.Assert(t, merged.Limit, be.Eq(20))

	// Nil cases
	betestify.Assert(t, a.MergeWith(nil), be.Eq(a))

	var nilSpec *r3.CursorSpec
	betestify.Assert(t, nilSpec.MergeWith(b), be.Eq(b))
}

func TestCursorSpec_String(t *testing.T) {
	betestify.Assert(t, r3.NewCursorAfter("tok", 10).String(), be_string.ContainingSubstring("after="))
	betestify.Assert(t, r3.NewCursorBefore("tok", 10).String(), be_string.ContainingSubstring("before="))
	betestify.Assert(t, r3.NewCursorFirst(10).String(), be_string.ContainingSubstring("first"))

	var nilSpec *r3.CursorSpec
	betestify.Assert(t, nilSpec.String(), be.Eq("no_cursor"))
}

func TestFinalizeCountCursor(t *testing.T) {
	items := []int{1, 2, 3}
	result, count := r3.FinalizeCountCursor(items)
	betestify.Assert(t, result, be.Eq(items))
	betestify.Assert(t, count, be.Eq(int64(-1)))
}

func TestQueryCursorCloneAndMerge(t *testing.T) {
	q := r3.Query{
		Cursor: r3.NewCursorAfter("tok", 25),
	}

	clone := q.Clone()
	betestify.Assert(t, clone.Cursor, be.Eq(q.Cursor))

	clone.Cursor.After = "changed"
	betestify.Assert(t, clone.Cursor.After, be.Not(be.Eq(q.Cursor.After)))

	other := r3.Query{
		Cursor: r3.NewCursorAfter("new", 50),
	}
	merged := q.MergeWith(other)
	betestify.Assert(t, merged.Cursor.After, be.Eq("new"))
	betestify.Assert(t, merged.Cursor.Limit, be.Eq(50))
}
