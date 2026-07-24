package canonical_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

func TestParseFilterOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected r3.FilterOperatorSpec
		wantErr  bool
	}{
		{"eq", r3.OperatorEq, false},
		{"ne", r3.OperatorNe, false},
		{"exists", r3.OperatorExists, false},
		{"gt", r3.OperatorGt, false},
		{"gte", r3.OperatorGte, false},
		{"lt", r3.OperatorLt, false},
		{"lte", r3.OperatorLte, false},
		{"between", r3.OperatorBetween, false},
		{"between_exc", r3.OperatorBetweenEx, false},
		{"between_exc_inc", r3.OperatorBetweenExInc, false},
		{"between_inc_exc", r3.OperatorBetweenIncEx, false},
		{"in", r3.OperatorIn, false},
		{"nin", r3.OperatorNotIn, false},
		{"like", r3.OperatorLike, false},
		{"notlike", r3.OperatorNotLike, false},
		{"ilike", r3.OperatorILike, false},
		{"weekday_in", r3.OperatorWeekdayIn, false},
		{"tod_between", r3.OperatorTimeOfDayBetween, false},
		// Invalid inputs
		{"", r3.OperatorUnspecified, true},
		{"unknown", r3.OperatorUnspecified, true},
		{"EQ", r3.OperatorUnspecified, true},
		{"==", r3.OperatorUnspecified, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := canonical.ParseFilterOperator(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, r3.OperatorUnspecified, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatFilterOperator(t *testing.T) {
	tests := []struct {
		input    r3.FilterOperatorSpec
		expected string
	}{
		{r3.OperatorEq, "eq"},
		{r3.OperatorNe, "ne"},
		{r3.OperatorExists, "exists"},
		{r3.OperatorGt, "gt"},
		{r3.OperatorGte, "gte"},
		{r3.OperatorLt, "lt"},
		{r3.OperatorLte, "lte"},
		{r3.OperatorBetween, "between"},
		{r3.OperatorBetweenEx, "between_exc"},
		{r3.OperatorBetweenExInc, "between_exc_inc"},
		{r3.OperatorBetweenIncEx, "between_inc_exc"},
		{r3.OperatorIn, "in"},
		{r3.OperatorNotIn, "nin"},
		{r3.OperatorLike, "like"},
		{r3.OperatorNotLike, "notlike"},
		{r3.OperatorILike, "ilike"},
		{r3.OperatorWeekdayIn, "weekday_in"},
		{r3.OperatorTimeOfDayBetween, "tod_between"},
		{r3.OperatorUnspecified, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := canonical.FormatFilterOperator(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterOperator_RoundTrip(t *testing.T) {
	// Every known operator should survive a format → parse round trip.
	operators := []r3.FilterOperatorSpec{
		r3.OperatorEq,
		r3.OperatorNe,
		r3.OperatorExists,
		r3.OperatorGt,
		r3.OperatorGte,
		r3.OperatorLt,
		r3.OperatorLte,
		r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx,
		r3.OperatorIn,
		r3.OperatorNotIn,
		r3.OperatorLike,
		r3.OperatorNotLike,
		r3.OperatorILike,
	}

	for _, op := range operators {
		name := canonical.FormatFilterOperator(op)
		require.NotEmpty(t, name, "FormatFilterOperator should return non-empty for %v", op)

		parsed, err := canonical.ParseFilterOperator(name)
		require.NoError(t, err, "ParseFilterOperator(%q) should succeed", name)
		assert.Equal(t, op, parsed, "round-trip failed for %v", op)
	}
}
