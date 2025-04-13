package gx_test

import (
	"testing"

	"github.com/amberpixels/r3/internal/gx"
	"github.com/stretchr/testify/assert"
)

// TestNewLookupAndFill.
func TestNewLookupAndFill(t *testing.T) {
	l := gx.NewLookup[string]()

	assert.False(t, l.Has("a"))
	assert.False(t, l.Has("b"))

	l.Add("a")

	assert.True(t, l.Has("a"))
	assert.False(t, l.Has("b"))
}

// TestNewLookupAndHas tests that NewLookup correctly initializes a lookup and Has
// returns true for the initial keys.
func TestNewLookupAndHas(t *testing.T) {
	l := gx.NewLookup("a", "b", "c")

	assert.True(t, l.Has("a"), "Expected key 'a' to be present")
	assert.True(t, l.Has("b"), "Expected key 'b' to be present")
	assert.True(t, l.Has("c"), "Expected key 'c' to be present")
	assert.False(t, l.Has("d"), "Expected key 'd' to not be present")
}

// TestAdd tests that the Add function correctly inserts a new key.
func TestAdd(t *testing.T) {
	l := gx.NewLookup("x")
	assert.True(t, l.Has("x"), "Expected key 'x' to be present")

	// Add a new key and verify it was added.
	l.Add("y")
	assert.True(t, l.Has("y"), "Expected key 'y' to be present after calling Add")
}

// TestLookupWithInts demonstrates that the generic Lookup works correctly for int types.
func TestLookupWithInts(t *testing.T) {
	l := gx.NewLookup(1, 2, 3)

	assert.True(t, l.Has(1), "Expected key 1 to be present")
	assert.True(t, l.Has(2), "Expected key 2 to be present")
	assert.True(t, l.Has(3), "Expected key 3 to be present")
	assert.False(t, l.Has(4), "Expected key 4 to not be present")

	// Test adding a new integer key.
	l.Add(4)
	assert.True(t, l.Has(4), "Expected key 4 to be present after calling Add")
}
