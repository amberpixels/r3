package r3_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
)

func TestWeekdayInSugar(t *testing.T) {
	f := r3.WeekdayIn("started_at", time.Saturday, time.Sunday)
	require.NotNil(t, f.Field)
	assert.Equal(t, "started_at", f.Field.String())
	assert.Equal(t, r3.OperatorWeekdayIn, f.Operator)
	assert.Equal(t, []time.Weekday{time.Saturday, time.Sunday}, f.Value)
}

func TestTimeOfDayBetweenSugar(t *testing.T) {
	f := r3.TimeOfDayBetween("started_at", 5*60, 12*60)
	assert.Equal(t, r3.OperatorTimeOfDayBetween, f.Operator)
	assert.Equal(t, []any{300, 720}, f.Value)
}

func TestWeekdaysValue(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    []time.Weekday
		wantErr bool
	}{
		{"native weekdays", []time.Weekday{time.Monday, time.Friday}, []time.Weekday{time.Monday, time.Friday}, false},
		{"ints", []int{0, 6}, []time.Weekday{time.Sunday, time.Saturday}, false},
		{"json float64", []float64{1, 3}, []time.Weekday{time.Monday, time.Wednesday}, false},
		{"empty", []int{}, []time.Weekday{}, false},
		{"not a slice", 3, nil, true},
		{"out of range high", []int{7}, nil, true},
		{"out of range low", []int{-1}, nil, true},
		{"fractional float", []float64{1.5}, nil, true},
		{"non-numeric element", []any{"monday"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r3.WeekdaysValue(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTimeOfDayBounds(t *testing.T) {
	tests := []struct {
		name           string
		value          any
		wantLo, wantHi int
		wantErr        bool
	}{
		{"ints", []any{300, 720}, 300, 720, false},
		{"json floats", []any{300.0, 720.0}, 300, 720, false},
		{"end of day", []any{1320, 1440}, 1320, 1440, false},
		{"wrapping bounds preserved", []any{1320, 300}, 1320, 300, false},
		{"not a pair", []any{300}, 0, 0, true},
		{"out of range", []any{0, 1441}, 0, 0, true},
		{"negative", []any{-1, 300}, 0, 0, true},
		{"fractional", []any{300.5, 720}, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi, err := r3.TimeOfDayBounds(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLo, lo)
			assert.Equal(t, tt.wantHi, hi)
		})
	}
}

func TestEvalWeekdayIn(t *testing.T) {
	// 2026-07-19 is a Sunday; 2026-07-20 is a Monday.
	sunday := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	monday := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)

	hit, err := r3.EvalWeekdayIn(sunday, []time.Weekday{time.Saturday, time.Sunday})
	require.NoError(t, err)
	assert.True(t, hit)

	miss, err := r3.EvalWeekdayIn(monday, []time.Weekday{time.Saturday, time.Sunday})
	require.NoError(t, err)
	assert.False(t, miss)

	_, err = r3.EvalWeekdayIn(sunday, "nope")
	require.Error(t, err)
}

func TestEvalTimeOfDayBetween(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 7, 19, h, m, 0, 0, time.UTC) }

	tests := []struct {
		name string
		t    time.Time
		lo   int
		hi   int
		want bool
	}{
		{"inside window", at(8, 0), 5 * 60, 12 * 60, true},
		{"lower bound inclusive", at(5, 0), 5 * 60, 12 * 60, true},
		{"upper bound exclusive", at(12, 0), 5 * 60, 12 * 60, false},
		{"before window", at(4, 59), 5 * 60, 12 * 60, false},
		{"wrap late night", at(23, 0), 22 * 60, 5 * 60, true},
		{"wrap early morning", at(3, 0), 22 * 60, 5 * 60, true},
		{"wrap miss midday", at(12, 0), 22 * 60, 5 * 60, false},
		{"wrap upper exclusive", at(5, 0), 22 * 60, 5 * 60, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r3.EvalTimeOfDayBetween(tt.t, []any{tt.lo, tt.hi})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
