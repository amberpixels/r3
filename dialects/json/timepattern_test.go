package r3json_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// TestTimePatternOperators_JSONRoundTrip verifies the time-component operators
// survive a full JSON encode/decode cycle: canonical name out, canonical name
// back, and a value that normalizes through the shared root helpers.
func TestTimePatternOperators_JSONRoundTrip(t *testing.T) {
	t.Run("weekday_in", func(t *testing.T) {
		in := r3.WeekdayIn("started_at", time.Saturday, time.Sunday)

		got := roundTrip(t, in)
		assert.Equal(t, r3.OperatorWeekdayIn, got.Operator)

		days, err := r3.WeekdaysValue(got.Value)
		require.NoError(t, err)
		assert.Equal(t, []time.Weekday{time.Saturday, time.Sunday}, days)
	})

	t.Run("tod_between", func(t *testing.T) {
		in := r3.TimeOfDayBetween("started_at", 1320, 300)

		got := roundTrip(t, in)
		assert.Equal(t, r3.OperatorTimeOfDayBetween, got.Operator)

		lo, hi, err := r3.TimeOfDayBounds(got.Value)
		require.NoError(t, err)
		assert.Equal(t, 1320, lo)
		assert.Equal(t, 300, hi)
	})
}

func roundTrip(t *testing.T, f *r3.FilterSpec) *r3.FilterSpec {
	t.Helper()

	jf, err := r3json.FilterToJSON(f)
	require.NoError(t, err)

	data, err := json.Marshal(jf)
	require.NoError(t, err)

	var decoded r3json.JSONFilter
	require.NoError(t, json.Unmarshal(data, &decoded))

	spec, err := r3json.JSONToFilter(&decoded)
	require.NoError(t, err)
	return spec
}
