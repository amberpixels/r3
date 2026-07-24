package r3when_test

import (
	"testing"
	"time"

	"github.com/amberpixels/years/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3when "github.com/amberpixels/r3/dialects/when"
)

const field = "started_at"

// evalFilters evaluates compiled when-filters against a time value using the
// same root primitives the real engines call, so the property test below pins
// the compiler to r3's actual operator semantics. Top-level filters are ANDed.
func evalFilters(fs r3.Filters, tm time.Time) bool {
	for _, f := range fs {
		if !evalFilter(f, tm) {
			return false
		}
	}
	return true
}

func evalFilter(f *r3.FilterSpec, tm time.Time) bool {
	if f == nil {
		return true
	}
	if len(f.And) > 0 {
		for _, c := range f.And {
			if !evalFilter(c, tm) {
				return false
			}
		}
		return true
	}
	if len(f.Or) > 0 {
		for _, c := range f.Or {
			if evalFilter(c, tm) {
				return true
			}
		}
		return false
	}

	switch f.Operator {
	case r3.OperatorWeekdayIn:
		ok, err := r3.EvalWeekdayIn(tm, f.Value)
		if err != nil {
			panic(err)
		}
		return ok
	case r3.OperatorTimeOfDayBetween:
		ok, err := r3.EvalTimeOfDayBetween(tm, f.Value)
		if err != nil {
			panic(err)
		}
		return ok
	default:
		panic("unexpected operator in compiled when-filter")
	}
}

// TestCompiledFiltersMatchPatternContains is the most important test in the
// package: for a dense grid of times across two weeks, the compiled r3 filters
// must accept exactly the instants years' own WeekPatterns.Contains accepts.
// This pins the semantic equivalence between years and r3 - including the
// cross-midnight attribution rule.
func TestCompiledFiltersMatchPatternContains(t *testing.T) {
	cases := map[string]schedule.WeekPatterns{
		"weekends":         mustParse(t, "weekends"),
		"weekdays":         mustParse(t, "weekdays"),
		"mornings":         mustParse(t, "mornings"),
		"nights (wraps)":   mustParse(t, "nights"),
		"sundays,mornings": mustParse(t, "sundays,mornings"),
		"single sunday":    {{Days: []time.Weekday{time.Sunday}}},
		// Cross-midnight span WITH a day constraint (shape 3): "Sunday nights"
		// 22:00 through Monday 05:00. The built-in vocabulary never emits this
		// shape, so build it explicitly.
		"sunday nights (cross-midnight + day)": {{
			Days:  []time.Weekday{time.Sunday},
			Spans: []schedule.DaySlot{{Start: schedule.TimeOfDay{Hour: 22}, End: schedule.TimeOfDay{Hour: 29}}},
		}},
		// Mixed union: weekends OR mornings.
		"weekends,mornings": mustParse(t, "weekends,mornings"),
		// Full-day wrapping span with a day constraint (24h window from 01:00):
		// covers Sun 01:00 through Mon 01:00. Exercises the shifted-weekday path
		// where the single-window shortcut would misfire.
		"24h wrap + day": {{
			Days:  []time.Weekday{time.Sunday},
			Spans: []schedule.DaySlot{{Start: schedule.TimeOfDay{Hour: 1}, End: schedule.TimeOfDay{Hour: 25}}},
		}},
		// >24h wrapping span with a day constraint.
		"26h wrap + day": {{
			Days:  []time.Weekday{time.Wednesday},
			Spans: []schedule.DaySlot{{Start: schedule.TimeOfDay{Hour: 1}, End: schedule.TimeOfDay{Hour: 27}}},
		}},
	}

	// A Sunday 00:00 UTC start; step 15 minutes across 14 days.
	start := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	require.Equal(t, time.Sunday, start.Weekday())

	for name, ps := range cases {
		t.Run(name, func(t *testing.T) {
			filters := r3when.Filters(field, ps)
			for step := range 14 * 24 * 4 {
				tm := start.Add(time.Duration(step) * 15 * time.Minute)
				want := ps.Contains(tm)
				got := evalFilters(filters, tm)
				assert.Equalf(t, want, got, "mismatch at %s", tm.Format("Mon 15:04"))
			}
		})
	}
}

func TestFilters_MatchAll(t *testing.T) {
	// Empty pattern set -> no filter.
	assert.Nil(t, r3when.Filters(field, nil))

	// A pattern with no days and no spans matches every instant, so the whole
	// union collapses to no filter.
	ps := schedule.WeekPatterns{{}, {Days: []time.Weekday{time.Monday}}}
	assert.Nil(t, r3when.Filters(field, ps))

	// A full-day (>=24h) wrapping span with no day constraint tiles the whole
	// timeline - it must collapse to no filter, not an empty window that matches
	// nothing (lo == hi%1440).
	fullDay := schedule.WeekPatterns{{
		Spans: []schedule.DaySlot{{Start: schedule.TimeOfDay{Hour: 1}, End: schedule.TimeOfDay{Hour: 25}}},
	}}
	assert.Nil(t, r3when.Filters(field, fullDay))
}

func TestFilters_WeekdayOnly(t *testing.T) {
	ps := schedule.WeekPatterns{{Days: []time.Weekday{time.Saturday, time.Sunday}}}
	got := r3when.Filters(field, ps)

	require.Len(t, got, 1)
	assert.Equal(t, r3.OperatorWeekdayIn, got[0].Operator)
	assert.Equal(t, []time.Weekday{time.Saturday, time.Sunday}, got[0].Value)
}

func TestFilters_TimeOfDayOnly(t *testing.T) {
	// A daypart-only pattern (mornings 05:00-12:00) -> a single TimeOfDayBetween.
	ps := schedule.WeekPatterns{{Spans: []schedule.DaySlot{{
		Start: schedule.TimeOfDay{Hour: 5}, End: schedule.TimeOfDay{Hour: 12},
	}}}}
	got := r3when.Filters(field, ps)

	require.Len(t, got, 1)
	assert.Equal(t, r3.OperatorTimeOfDayBetween, got[0].Operator)
	assert.Equal(t, []any{300, 720}, got[0].Value)
}

func TestFilters_WeekdayAndTimeOfDay(t *testing.T) {
	// Days + non-wrapping span -> And(WeekdayIn, TimeOfDayBetween).
	ps := schedule.WeekPatterns{{
		Days:  []time.Weekday{time.Monday},
		Spans: []schedule.DaySlot{{Start: schedule.TimeOfDay{Hour: 9}, End: schedule.TimeOfDay{Hour: 17}}},
	}}
	got := r3when.Filters(field, ps)

	require.Len(t, got, 1)
	require.Len(t, got[0].And, 2)
	assert.Equal(t, r3.OperatorWeekdayIn, got[0].And[0].Operator)
	assert.Equal(t, r3.OperatorTimeOfDayBetween, got[0].And[1].Operator)
	assert.Equal(t, []any{540, 1020}, got[0].And[1].Value)
}

func TestParse_HappyPath(t *testing.T) {
	got, err := r3when.Parse(field, "weekends")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, r3.OperatorWeekdayIn, got[0].Operator)
}

func TestParse_UnknownTermError(t *testing.T) {
	_, err := r3when.Parse(field, "gibberish")
	require.Error(t, err)
	// The years message lists the known terms; it is passed through verbatim.
	assert.Contains(t, err.Error(), "known terms")
}

func TestParse_InvalidFieldRejected(t *testing.T) {
	_, err := r3when.Parse("started_at; DROP TABLE", "weekends")
	require.Error(t, err)
}

func mustParse(t *testing.T, expr string) schedule.WeekPatterns {
	t.Helper()
	ps, err := schedule.ParsePatterns(expr)
	require.NoError(t, err)
	return ps
}
