package r3

import "time"

// A time-bucket group key truncates a time-valued field to a fixed calendar unit
// (day, week, month, ...) so an aggregation can group a raw timestamp column into
// a chart-friendly series ("counts per day"). It is the aggregation-side analog
// of the closed-form time-component filter operators ([WeekdayIn] /
// [TimeOfDayBetween]): the core knows how to truncate an extracted time component
// to a fixed unit, but has no vocabulary ("last quarter", "fiscal year") of its
// own - any such vocabulary is a dialect concern.
//
// Semantics contract: a bucket truncates the field's stored wall-clock value
// as-is. No engine performs timezone conversion (identical to the WeekdayIn /
// TimeOfDayBetween contract). Store UTC (or a normalized local wall-clock) for
// identical results across backends. Week buckets start on ISO-8601 Monday on
// every backend.

// BucketUnit is the closed set of calendar truncation units for a time-bucket
// group key (see [BucketSpec]).
type BucketUnit int8

const (
	// BucketHour truncates to the top of the hour.
	BucketHour BucketUnit = iota + 1
	// BucketDay truncates to midnight of the day.
	BucketDay
	// BucketWeek truncates to midnight of the ISO-8601 week start (Monday).
	BucketWeek
	// BucketMonth truncates to midnight of the first day of the month.
	BucketMonth
	// BucketYear truncates to midnight of January 1st.
	BucketYear
)

// String returns the canonical lowercase name of the unit (also the Mongo
// $dateTrunc unit name).
func (u BucketUnit) String() string {
	switch u {
	case BucketHour:
		return "hour"
	case BucketDay:
		return "day"
	case BucketWeek:
		return "week"
	case BucketMonth:
		return "month"
	case BucketYear:
		return "year"
	default:
		return strUnspecified
	}
}

// Valid reports whether u is one of the defined bucket units.
func (u BucketUnit) Valid() bool { return u >= BucketHour && u <= BucketYear }

// BucketSpec is a derived group key: the truncation of a time-valued field to
// Unit, appearing in each [AggregateRow] under Alias (also the name Sorts and
// Having refer to). It participates in GROUP BY alongside [Query.GroupBy] fields.
type BucketSpec struct {
	Field *FieldSpec
	Unit  BucketUnit
	Alias string
}

// Clone returns a deep copy of the spec.
func (b *BucketSpec) Clone() *BucketSpec {
	if b == nil {
		return nil
	}
	clone := *b
	if b.Field != nil {
		clone.Field = b.Field.Clone()
	}
	return &clone
}

// Buckets is a slice of *BucketSpec.
type Buckets []*BucketSpec

// Clone returns a safe full-clone of the buckets list.
func (bs Buckets) Clone() Buckets {
	if bs == nil {
		return nil
	}
	cloned := make(Buckets, len(bs))
	for i, b := range bs {
		cloned[i] = b.Clone()
	}
	return cloned
}

// Bucket builds a time-bucket group key: the value of field truncated to unit,
// returned under alias.
//
//	r3.Query{Buckets: r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")}}
func Bucket(field string, unit BucketUnit, alias string) *BucketSpec {
	return &BucketSpec{Field: NewFieldSpec(field), Unit: unit, Alias: alias}
}

// TruncateToBucket truncates t to the start of the calendar unit, in t's own
// location (no timezone conversion). Week starts on ISO-8601 Monday. An unknown
// unit returns t unchanged (structural validation rejects it before any engine
// reaches this). In-memory engines (the file engine) call it so the truncation
// rule lives in exactly one place - the analog of [EvalWeekdayIn].
func TruncateToBucket(t time.Time, u BucketUnit) time.Time {
	y, mo, d := t.Date()
	loc := t.Location()
	switch u {
	case BucketHour:
		return time.Date(y, mo, d, t.Hour(), 0, 0, 0, loc)
	case BucketDay:
		return time.Date(y, mo, d, 0, 0, 0, 0, loc)
	case BucketWeek:
		// ISO-8601 week starts Monday. Go's Weekday is Sunday=0..Saturday=6, so
		// days since Monday is (weekday + 6) % 7 (Monday->0, Sunday->6).
		const daysPerWeek = 7
		daysSinceMonday := (int(t.Weekday()) + daysPerWeek - 1) % daysPerWeek
		midnight := time.Date(y, mo, d, 0, 0, 0, 0, loc)
		return midnight.AddDate(0, 0, -daysSinceMonday)
	case BucketMonth:
		return time.Date(y, mo, 1, 0, 0, 0, 0, loc)
	case BucketYear:
		return time.Date(y, 1, 1, 0, 0, 0, 0, loc)
	default:
		return t
	}
}
