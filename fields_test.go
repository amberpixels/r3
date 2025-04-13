package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
)

func TestFieldsMergeWith(t *testing.T) {
	testCases := []struct {
		name     string
		a        r3.Fields
		b        r3.Fields
		expected r3.Fields
	}{
		{
			name: "Non-overlapping merge",
			a:    r3.Fields{r3.ColumnField("a"), r3.ColumnField("b")},
			b:    r3.Fields{r3.ColumnField("c"), r3.ColumnField("d")},
			expected: r3.Fields{
				r3.ColumnField("a"),
				r3.ColumnField("b"),
				r3.ColumnField("c"),
				r3.ColumnField("d"),
			},
		},
		{
			name: "Overlapping merge",
			a:    r3.Fields{r3.ColumnField("a"), r3.ColumnField("b")},
			b:    r3.Fields{r3.ColumnField("b"), r3.ColumnField("c")},
			expected: r3.Fields{
				r3.ColumnField("a"),
				r3.ColumnField("b"),
				r3.ColumnField("c"),
			},
		},
		{
			name:     "Empty a slice",
			a:        r3.Fields{},
			b:        r3.Fields{r3.ColumnField("a"), r3.ColumnField("b")},
			expected: r3.Fields{r3.ColumnField("a"), r3.ColumnField("b")},
		},
		{
			name:     "Empty b slice",
			a:        r3.Fields{r3.ColumnField("a"), r3.ColumnField("b")},
			b:        r3.Fields{},
			expected: r3.Fields{r3.ColumnField("a"), r3.ColumnField("b")},
		},
		{
			name:     "Both slices empty",
			a:        r3.Fields{},
			b:        r3.Fields{},
			expected: r3.Fields{},
		},
		{
			name: "Existed duplicates are respected",
			a:    r3.Fields{r3.ColumnField("a"), r3.ColumnField("b"), r3.ColumnField("b")},
			b:    r3.Fields{r3.ColumnField("b"), r3.ColumnField("c")},
			expected: r3.Fields{
				r3.ColumnField("a"),
				r3.ColumnField("b"),
				r3.ColumnField("b"),
				r3.ColumnField("c"),
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
		input    r3.Fields
		expected r3.Fields
	}{
		{
			name:     "No duplicates",
			input:    r3.Fields{r3.ColumnField("a"), r3.ColumnField("b"), r3.ColumnField("c")},
			expected: r3.Fields{r3.ColumnField("a"), r3.ColumnField("b"), r3.ColumnField("c")},
		},
		{
			name: "Duplicates present",
			input: r3.Fields{
				r3.ColumnField("a"),
				r3.ColumnField("b"),
				r3.ColumnField("b"),
				r3.ColumnField("a"),
				r3.ColumnField("c"),
				r3.ColumnField("c"),
			},
			expected: r3.Fields{r3.ColumnField("a"), r3.ColumnField("b"), r3.ColumnField("c")},
		},
		{
			name:     "All duplicates",
			input:    r3.Fields{r3.ColumnField("a"), r3.ColumnField("a"), r3.ColumnField("a")},
			expected: r3.Fields{r3.ColumnField("a")},
		},
		{
			name:     "Empty slice",
			input:    r3.Fields{},
			expected: r3.Fields{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Work on a copy of tc.input since Dedupe mutates the slice.
			input := make(r3.Fields, len(tc.input))
			copy(input, tc.input)

			input.Dedupe()
			assert.Equal(t, tc.expected, input, "Dedupe result did not match expected output")
		})
	}
}
