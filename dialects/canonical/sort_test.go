package canonical_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

func TestParseSortDirection(t *testing.T) {
	tests := []struct {
		input    string
		expected r3.SortDirection
	}{
		{"asc", r3.SortDirectionAsc},
		{"desc", r3.SortDirectionDesc},
		{"", r3.SortDirectionUnspecified},
		{"unknown", r3.SortDirectionUnspecified},
		{"ASC", r3.SortDirectionUnspecified},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := canonical.ParseSortDirection(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSortDirection(t *testing.T) {
	tests := []struct {
		input    r3.SortDirection
		expected string
	}{
		{r3.SortDirectionAsc, "asc"},
		{r3.SortDirectionDesc, "desc"},
		{r3.SortDirectionUnspecified, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := canonical.FormatSortDirection(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortDirection_RoundTrip(t *testing.T) {
	directions := []r3.SortDirection{
		r3.SortDirectionAsc,
		r3.SortDirectionDesc,
	}

	for _, d := range directions {
		name := canonical.FormatSortDirection(d)
		assert.NotEmpty(t, name)
		parsed := canonical.ParseSortDirection(name)
		assert.Equal(t, d, parsed, "round-trip failed for %v", d)
	}
}

func TestParseNullsPosition(t *testing.T) {
	tests := []struct {
		input    string
		expected r3.SortNullsPosition
	}{
		{"first", r3.NullsPositionFirst},
		{"last", r3.NullsPositionLast},
		{"", r3.NullsPositionNotSpecified},
		{"unknown", r3.NullsPositionNotSpecified},
		{"FIRST", r3.NullsPositionNotSpecified},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := canonical.ParseNullsPosition(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatNullsPosition(t *testing.T) {
	tests := []struct {
		input    r3.SortNullsPosition
		expected string
	}{
		{r3.NullsPositionFirst, "first"},
		{r3.NullsPositionLast, "last"},
		{r3.NullsPositionNotSpecified, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := canonical.FormatNullsPosition(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNullsPosition_RoundTrip(t *testing.T) {
	positions := []r3.SortNullsPosition{
		r3.NullsPositionFirst,
		r3.NullsPositionLast,
	}

	for _, p := range positions {
		name := canonical.FormatNullsPosition(p)
		assert.NotEmpty(t, name)
		parsed := canonical.ParseNullsPosition(name)
		assert.Equal(t, p, parsed, "round-trip failed for %v", p)
	}
}
