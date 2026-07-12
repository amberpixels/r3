package canonical

import (
	"github.com/amberpixels/r3"
)

// ParseSortDirection parses "asc"/"desc"; anything else is
// SortDirectionUnspecified.
func ParseSortDirection(s string) r3.SortDirection {
	switch s {
	case "asc":
		return r3.SortDirectionAsc
	case "desc":
		return r3.SortDirectionDesc
	default:
		return r3.SortDirectionUnspecified
	}
}

// FormatSortDirection returns "asc", "desc", or "" for unspecified.
func FormatSortDirection(d r3.SortDirection) string {
	switch d {
	case r3.SortDirectionAsc:
		return "asc"
	case r3.SortDirectionDesc:
		return "desc"
	case r3.SortDirectionUnspecified:
		return ""
	default:
		return ""
	}
}

// ParseNullsPosition parses "first"/"last"; anything else is
// NullsPositionNotSpecified.
func ParseNullsPosition(s string) r3.SortNullsPosition {
	switch s {
	case "first":
		return r3.NullsPositionFirst
	case "last":
		return r3.NullsPositionLast
	default:
		return r3.NullsPositionNotSpecified
	}
}

// FormatNullsPosition returns "first", "last", or "" for unspecified.
func FormatNullsPosition(p r3.SortNullsPosition) string {
	switch p {
	case r3.NullsPositionFirst:
		return "first"
	case r3.NullsPositionLast:
		return "last"
	case r3.NullsPositionNotSpecified:
		return ""
	default:
		return ""
	}
}
