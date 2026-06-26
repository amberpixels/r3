package metrics

import (
	"time"

	"github.com/amberpixels/years"
)

// BucketSize defines the granularity of time-based metric bucketing.
type BucketSize string

const (
	BucketMinutely BucketSize = "minutely"
	BucketHourly   BucketSize = "hourly"
	BucketDaily    BucketSize = "daily"
	BucketWeekly   BucketSize = "weekly"
	BucketMonthly  BucketSize = "monthly"
)

// ComputeBucket truncates a timestamp to the given bucket boundary
// and returns it as an ISO8601/RFC3339 string. Truncation is delegated to the
// years library so the boundary semantics stay consistent across the codebase.
func ComputeBucket(t time.Time, size BucketSize) string {
	t = t.UTC()
	mt := years.Mutate(&t)
	switch size {
	case BucketMinutely:
		mt.TruncateToMinute()
	case BucketHourly:
		mt.TruncateToHour()
	case BucketDaily:
		mt.TruncateToDay()
	case BucketWeekly:
		// ISO-8601 weeks start on Monday.
		mt.TruncateToWeek(time.Monday)
	case BucketMonthly:
		mt.TruncateToMonth()
	default:
		mt.TruncateToDay()
	}
	return mt.Time().Format(time.RFC3339)
}

// TimeRange defines a time window for querying metrics.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// LastMinutes returns a TimeRange covering the last n minutes.
func LastMinutes(n int) TimeRange {
	now := years.Now().UTC()
	return TimeRange{From: now.Add(-time.Duration(n) * time.Minute), To: now}
}

// LastHours returns a TimeRange covering the last n hours.
func LastHours(n int) TimeRange {
	now := years.Now().UTC()
	return TimeRange{From: now.Add(-time.Duration(n) * time.Hour), To: now}
}

// LastDays returns a TimeRange covering the last n days.
func LastDays(n int) TimeRange {
	now := years.Now().UTC()
	return TimeRange{From: now.AddDate(0, 0, -n), To: now}
}

// Common time range constants.
const (
	hoursPerDay  = 24
	daysPerWeek  = 7
	daysPerMonth = 30
	daysPerYear  = 365
)

// Last24Hours returns a TimeRange covering the last 24 hours.
func Last24Hours() TimeRange { return LastHours(hoursPerDay) }

// LastWeek returns a TimeRange covering the last 7 days.
func LastWeek() TimeRange { return LastDays(daysPerWeek) }

// Last30Days returns a TimeRange covering the last 30 days.
func Last30Days() TimeRange { return LastDays(daysPerMonth) }

// LastMonth returns a TimeRange covering the last calendar month.
func LastMonth() TimeRange {
	now := years.Now().UTC()
	return TimeRange{From: now.AddDate(0, -1, 0), To: now}
}

// LastYear returns a TimeRange covering the last 365 days.
func LastYear() TimeRange { return LastDays(daysPerYear) }
