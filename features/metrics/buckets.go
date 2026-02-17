package metrics

import "time"

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
// and returns it as an ISO8601/RFC3339 string.
func ComputeBucket(t time.Time, size BucketSize) string {
	t = t.UTC()
	switch size {
	case BucketMinutely:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC).Format(time.RFC3339)
	case BucketHourly:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC).Format(time.RFC3339)
	case BucketDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	case BucketWeekly:
		// ISO week: Monday is day 0.
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	case BucketMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	default:
		// Default to daily
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
}

// TimeRange defines a time window for querying metrics.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// LastMinutes returns a TimeRange covering the last n minutes.
func LastMinutes(n int) TimeRange {
	now := time.Now().UTC()
	return TimeRange{From: now.Add(-time.Duration(n) * time.Minute), To: now}
}

// LastHours returns a TimeRange covering the last n hours.
func LastHours(n int) TimeRange {
	now := time.Now().UTC()
	return TimeRange{From: now.Add(-time.Duration(n) * time.Hour), To: now}
}

// LastDays returns a TimeRange covering the last n days.
func LastDays(n int) TimeRange {
	now := time.Now().UTC()
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
	now := time.Now().UTC()
	return TimeRange{From: now.AddDate(0, -1, 0), To: now}
}

// LastYear returns a TimeRange covering the last 365 days.
func LastYear() TimeRange { return LastDays(daysPerYear) }
