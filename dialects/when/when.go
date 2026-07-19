package r3when

import (
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/years/schedule"
)

const (
	daysInWeek    = 7
	minutesPerDay = 24 * 60
)

// Parse resolves a vocabulary expression ("weekends", "mornings,evenings") into
// r3 filters against field, using the default years vocabulary. The years error
// (which lists the known terms) is passed through unchanged - it is user-facing
// and belongs verbatim in an HTTP 400.
func Parse(field, expr string) (r3.Filters, error) {
	return ParseWith(schedule.DefaultVocabulary(), field, expr)
}

// ParseWith is [Parse] with a caller-supplied vocabulary, for apps that add
// domain terms or a custom day partition.
func ParseWith(v schedule.Vocabulary, field, expr string) (r3.Filters, error) {
	if err := r3.ValidateIdentifier(field); err != nil {
		return nil, err
	}
	patterns, err := v.ParsePatterns(expr)
	if err != nil {
		return nil, err
	}
	return Filters(field, patterns), nil
}

// Filters compiles a union of week patterns into r3 filters against field. The
// union is an OR of per-pattern groups; each pattern is a conjunction of its
// weekday set and its time-of-day spans. An empty pattern set - or any pattern
// that matches every instant - compiles to nil (no filter), since an OR with a
// match-everything member matches everything.
//
// The output uses only root filter vocabulary (WeekdayIn / TimeOfDayBetween and
// And/Or groups), so it serializes through every dialect and runs on any engine
// that lowers those operators.
func Filters(field string, ps schedule.WeekPatterns) r3.Filters {
	if len(ps) == 0 {
		return nil
	}

	groups := make(r3.Filters, 0, len(ps))
	for _, p := range ps {
		g := compilePattern(field, p)
		if g == nil {
			// This pattern matches every instant, so the whole union does too.
			return nil
		}
		groups = append(groups, g)
	}

	if len(groups) == 1 {
		return groups
	}
	return r3.Filters{r3.Or(groups...)}
}

// compilePattern lowers one WeekPattern to a single *FilterSpec, or nil when the
// pattern matches every instant (empty Days and empty Spans). It is correct for
// every shape the type allows, including cross-midnight spans and combinations
// the built-in vocabulary never emits.
func compilePattern(field string, p schedule.WeekPattern) *r3.FilterSpec {
	// No spans: a pure weekday constraint (or match-all when Days is empty too).
	if len(p.Spans) == 0 {
		if len(p.Days) == 0 {
			return nil
		}
		return r3.WeekdayIn(field, p.Days...)
	}

	// members are the OR alternatives of this pattern. Non-wrapping spans share a
	// single weekday constraint (factored out); cross-midnight spans each carry
	// their own coupled constraint because the day attribution differs per side.
	var members []*r3.FilterSpec
	var nonWrapTODs []*r3.FilterSpec

	for _, span := range p.Spans {
		lo := totalMinutes(span.Start)
		hi := totalMinutes(span.End)

		if hi <= minutesPerDay {
			nonWrapTODs = append(nonWrapTODs, r3.TimeOfDayBetween(field, lo, hi))
			continue
		}

		// Cross-midnight span (End >= 24:00).
		if len(p.Days) == 0 {
			// A window spanning a full 24h (or more) with no day constraint tiles
			// the whole timeline - it matches every instant, collapsing the union
			// to no filter. (A single wrapping TimeOfDayBetween cannot express this:
			// lo == hi%1440 reads as an empty window, not a full one.)
			if hi-lo >= minutesPerDay {
				return nil
			}
			// Otherwise a single wrapping window is cheaper and exact.
			members = append(members, r3.TimeOfDayBetween(field, lo, hi%minutesPerDay))
			continue
		}

		// With a day constraint, the span is attributed to the day it starts on
		// (years' rule: "sunday night" includes Monday 01:00). That lowers to a
		// disjunction over the start day [lo, midnight) and the next day
		// [midnight, hi-24h).
		members = append(members,
			r3.And(r3.WeekdayIn(field, p.Days...), r3.TimeOfDayBetween(field, lo, minutesPerDay)),
			r3.And(r3.WeekdayIn(field, shiftWeekdays(p.Days)...), r3.TimeOfDayBetween(field, 0, hi-minutesPerDay)),
		)
	}

	if len(nonWrapTODs) > 0 {
		tod := orGroup(nonWrapTODs)
		factored := tod
		if len(p.Days) > 0 {
			factored = r3.And(r3.WeekdayIn(field, p.Days...), tod)
		}
		// Keep the factored, common-case group first for readable output.
		members = append([]*r3.FilterSpec{factored}, members...)
	}

	return orGroup(members)
}

// totalMinutes is a time-of-day as minutes from its day's midnight (>= 1440 for
// the cross-midnight hours schedule.TimeOfDay allows in a span End).
func totalMinutes(t schedule.TimeOfDay) int {
	return t.Hour*60 + t.Minute
}

// shiftWeekdays returns each weekday advanced by one day (mod 7), the next-day
// set for a cross-midnight span's second half.
func shiftWeekdays(days []time.Weekday) []time.Weekday {
	shifted := make([]time.Weekday, len(days))
	for i, d := range days {
		shifted[i] = time.Weekday((int(d) + 1) % daysInWeek)
	}
	return shifted
}

// orGroup returns fs joined by OR, unwrapping a single element so trivial groups
// stay flat.
func orGroup(fs []*r3.FilterSpec) *r3.FilterSpec {
	if len(fs) == 1 {
		return fs[0]
	}
	return r3.Or(fs...)
}
