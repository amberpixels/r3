package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
)

func TestFieldsMergeWith(t *testing.T) {
	a := r3.NewFieldSpec("a")
	b := r3.NewFieldSpec("b")
	c := r3.NewFieldSpec("c")
	d := r3.NewFieldSpec("d")

	testCases := []struct {
		name     string
		a        r3.Fields
		b        r3.Fields
		expected r3.Fields
	}{
		{
			name: "Non-overlapping merge",
			a:    r3.Fields{a, b},
			b:    r3.Fields{c, d},
			expected: r3.Fields{
				a,
				b,
				c,
				d,
			},
		},
		{
			name: "Overlapping merge",
			a:    r3.Fields{a, b},
			b:    r3.Fields{b, c},
			expected: r3.Fields{
				a,
				b,
				c,
			},
		},
		{
			name:     "Empty a slice",
			a:        r3.Fields{},
			b:        r3.Fields{a, b},
			expected: r3.Fields{a, b},
		},
		{
			name:     "Empty b slice",
			a:        r3.Fields{a, b},
			b:        r3.Fields{},
			expected: r3.Fields{a, b},
		},
		{
			name:     "Both slices empty",
			a:        r3.Fields{},
			b:        r3.Fields{},
			expected: r3.Fields{},
		},
		{
			name: "Existed duplicates are respected",
			a:    r3.Fields{a, b, b},
			b:    r3.Fields{b, c},
			expected: r3.Fields{
				a,
				b,
				b,
				c,
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
	a := r3.NewFieldSpec("a")
	b := r3.NewFieldSpec("b")
	c := r3.NewFieldSpec("c")

	testCases := []struct {
		name     string
		input    r3.Fields
		expected r3.Fields
	}{
		{
			name:     "No duplicates",
			input:    r3.Fields{a, b, c},
			expected: r3.Fields{a, b, c},
		},
		{
			name: "Duplicates present",
			input: r3.Fields{
				a,
				b,
				b,
				a,
				c,
				c,
			},
			expected: r3.Fields{a, b, c},
		},
		{
			name:     "All duplicates",
			input:    r3.Fields{a, a, a},
			expected: r3.Fields{a},
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
