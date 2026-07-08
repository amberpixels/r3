package r3

// SortSpec represents a single sort criteria.
type SortSpec struct {
	Column    *FieldSpec
	Direction SortDirection // Asc | Desc | Unspecified (default)

	NullsPosition SortNullsPosition // First | Last | Unspecified
}

// String returns string representation of the sort spec.
func (s *SortSpec) String() string {
	// For debugging - use a simple format
	str := s.Column.String() + " " + s.Direction.String()
	if s.NullsPosition != NullsPositionNotSpecified {
		str += " " + s.NullsPosition.String()
	}
	return str
}

// GetDirection returns the sort direction.
func (s *SortSpec) GetDirection() SortDirection { return s.Direction }

// GetCriteria returns the sort criteria.
func (s *SortSpec) GetCriteria() *FieldSpec { return s.Column }

// Clone returns a new pointer to the sort spec (safe and deep clone).
func (s *SortSpec) Clone() *SortSpec {
	var clone SortSpec
	clone.Column = s.Column.Clone()
	clone.Direction = s.Direction
	clone.NullsPosition = s.NullsPosition

	return &clone
}

// NewSortSpec is the main SortSpec constructor.
func NewSortSpec(col *FieldSpec, direction SortDirection, nullsPositionArg ...SortNullsPosition) *SortSpec {
	s := &SortSpec{
		Column:    col,
		Direction: direction,
	}

	if len(nullsPositionArg) > 0 {
		s.NullsPosition = nullsPositionArg[0]
	}

	return s
}

// NewSortAscSpec returns sort by a given field ordered ASC.
func NewSortAscSpec(col *FieldSpec) *SortSpec {
	return NewSortSpec(col, SortDirectionAsc)
}

// NewSortDescSpec returns sort by a given field ordered DESC.
func NewSortDescSpec(col *FieldSpec) *SortSpec {
	return NewSortSpec(col, SortDirectionDesc)
}

// Sorts represents a list of *SortSpec.
type Sorts []*SortSpec

// MergeWith merges other sorts into ours.
func (sorts Sorts) MergeWith(other Sorts) Sorts { return mergeWith(sorts, other) }

// Clone returns a cloned list of given sorts.
func (sorts Sorts) Clone() Sorts {
	cloned := make(Sorts, len(sorts))
	for i, s := range sorts {
		cloned[i] = s.Clone()
	}
	return cloned
}

// SortNullsPosition represents a position of nulls in sort (first or last).
type SortNullsPosition int8

const (
	// NullsPositionNotSpecified means that nulls position won't be specified in the query.
	NullsPositionNotSpecified SortNullsPosition = iota

	// NullsPositionFirst means that nulls will be placed first in the result.
	NullsPositionFirst

	// NullsPositionLast means that nulls will be placed last in the result.
	NullsPositionLast
)

// String is implemented for debugging purposes, so the SortNullsPosition is a fmt.Stringer.
func (s SortNullsPosition) String() string {
	switch s {
	case NullsPositionFirst:
		return "nulls first"
	case NullsPositionLast:
		return "nulls last"
	case NullsPositionNotSpecified:
		fallthrough
	default:
		return ""
	}
}

// SortDirection represents a direction of sorting.
type SortDirection int8

const (
	// SortDirectionUnspecified means that direction is not specified by user and will be fallback to default.
	SortDirectionUnspecified SortDirection = iota
	// SortDirectionAsc means that sorting will be done in ascending order.
	SortDirectionAsc
	// SortDirectionDesc means that sorting will be done in descending order.
	SortDirectionDesc
)

// String is implemented for debugging purposes, so the SortDirection is a fmt.Stringer.
func (s SortDirection) String() string {
	switch s {
	case SortDirectionAsc:
		return "asc"
	case SortDirectionDesc:
		return "desc"
	case SortDirectionUnspecified:
		fallthrough
	default:
		return strUnspecified
	}
}
