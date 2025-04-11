package r3atoms_test

import (
	"testing"

	. "github.com/amberpixels/r3/atoms"
	"github.com/stretchr/testify/assert"
)

func TestFieldsMergeWith(t *testing.T) {
	testCases := []struct {
		name     string
		a        Fields
		b        Fields
		expected Fields
	}{
		{
			name: "Non-overlapping merge",
			a:    Fields{ColumnField("a"), ColumnField("b")},
			b:    Fields{ColumnField("c"), ColumnField("d")},
			expected: Fields{
				ColumnField("a"),
				ColumnField("b"),
				ColumnField("c"),
				ColumnField("d"),
			},
		},
		{
			name: "Overlapping merge",
			a:    Fields{ColumnField("a"), ColumnField("b")},
			b:    Fields{ColumnField("b"), ColumnField("c")},
			expected: Fields{
				ColumnField("a"),
				ColumnField("b"),
				ColumnField("c"),
			},
		},
		{
			name:     "Empty a slice",
			a:        Fields{},
			b:        Fields{ColumnField("a"), ColumnField("b")},
			expected: Fields{ColumnField("a"), ColumnField("b")},
		},
		{
			name:     "Empty b slice",
			a:        Fields{ColumnField("a"), ColumnField("b")},
			b:        Fields{},
			expected: Fields{ColumnField("a"), ColumnField("b")},
		},
		{
			name:     "Both slices empty",
			a:        Fields{},
			b:        Fields{},
			expected: Fields{},
		},
		{
			name: "Existed duplicates are respected",
			a:    Fields{ColumnField("a"), ColumnField("b"), ColumnField("b")},
			b:    Fields{ColumnField("b"), ColumnField("c")},
			expected: Fields{
				ColumnField("a"),
				ColumnField("b"),
				ColumnField("b"),
				ColumnField("c"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			merged := tc.a.MergeWith(tc.b)
			assert.Equal(t, tc.expected, merged, "Merged fields did not match expected result")
		})
	}
}

// TestFieldsDedupe uses table testing to cover various scenarios for Dedupe.
func TestFieldsDedupe(t *testing.T) {
	testCases := []struct {
		name     string
		input    Fields
		expected Fields
	}{
		{
			name:     "No duplicates",
			input:    Fields{ColumnField("a"), ColumnField("b"), ColumnField("c")},
			expected: Fields{ColumnField("a"), ColumnField("b"), ColumnField("c")},
		},
		{
			name: "Duplicates present",
			input: Fields{
				ColumnField("a"),
				ColumnField("b"),
				ColumnField("b"),
				ColumnField("a"),
				ColumnField("c"),
				ColumnField("c"),
			},
			expected: Fields{ColumnField("a"), ColumnField("b"), ColumnField("c")},
		},
		{
			name:     "All duplicates",
			input:    Fields{ColumnField("a"), ColumnField("a"), ColumnField("a")},
			expected: Fields{ColumnField("a")},
		},
		{
			name:     "Empty slice",
			input:    Fields{},
			expected: Fields{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Work on a copy of tc.input since Dedupe mutates the slice.
			input := make(Fields, len(tc.input))
			copy(input, tc.input)

			input.Dedupe()
			assert.Equal(t, tc.expected, input, "Dedupe result did not match expected output")
		})
	}
}
