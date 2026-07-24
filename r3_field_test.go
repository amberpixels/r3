package r3_test

import (
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
)

func TestValidateIdentifier(t *testing.T) {
	valid := []string{
		"id",
		"name",
		"user_name",
		"_private",
		"CamelCase",
		"col123",
		"user.name",
		"user.profile.name",
		"a.b.c.d",
		"_a._b",
		"T1.col_2",
	}

	for _, s := range valid {
		t.Run("valid/"+s, func(t *testing.T) {
			be.AssertThat(t, r3.ValidateIdentifier(s), be.Succeed())
		})
	}

	invalid := []string{
		"",
		"1col",                         // starts with digit
		"a b",                          // space
		"x;y",                          // semicolon
		"col--",                        // double dash (SQL comment)
		"table.*",                      // wildcard
		"a..b",                         // empty segment
		".leading",                     // leading dot
		"trailing.",                    // trailing dot
		"col name",                     // space in name
		"id; DROP TABLE users--",       // SQL injection attempt
		"1=1) UNION SELECT password--", // SQL injection attempt
		`"quoted"`,                     // already quoted
		"col\x00name",                  // null byte
		"(subquery)",                   // parentheses
		"a,b",                          // comma
		"a=b",                          // equals
		"col'name",                     // single quote
	}

	for _, s := range invalid {
		t.Run("invalid/"+s, func(t *testing.T) {
			// be.MatchError is the errors.Is-style sentinel check (implies non-nil).
			be.AssertThat(t, r3.ValidateIdentifier(s), be.MatchError(r3.ErrInvalidIdentifier))
		})
	}
}

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
			be.AssertThat(t, merged, be.Eq(tc.expected), "Merged fields did not match expected result")
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
			be.AssertThat(t, input, be.Eq(tc.expected), "Dedupe result did not match expected output")
		})
	}
}
