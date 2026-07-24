package r3

// SortSpec is a single sort criterion.
type SortSpec struct {
	Column    *FieldSpec
	Direction SortDirection // Asc | Desc | Unspecified (default)

	NullsPosition SortNullsPosition // First | Last | Unspecified
}

// NewSortSpec is the primary SortSpec constructor.
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

// NewSortAscSpec sorts by col ascending.
func NewSortAscSpec(col *FieldSpec) *SortSpec {
	return NewSortSpec(col, SortDirectionAsc)
}

// NewSortDescSpec sorts by col descending.
func NewSortDescSpec(col *FieldSpec) *SortSpec {
	return NewSortSpec(col, SortDirectionDesc)
}

// String returns a debug representation of the sort spec.
func (s *SortSpec) String() string {
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

// Clone returns a deep copy of the sort spec.
func (s *SortSpec) Clone() *SortSpec {
	var clone SortSpec
	clone.Column = s.Column.Clone()
	clone.Direction = s.Direction
	clone.NullsPosition = s.NullsPosition

	return &clone
}

// Sorts is a list of *SortSpec.
type Sorts []*SortSpec

// MergeWith merges other sorts into these.
func (sorts Sorts) MergeWith(other Sorts) Sorts { return mergeWith(sorts, other) }

// Clone returns a deep copy of the sorts.
func (sorts Sorts) Clone() Sorts {
	cloned := make(Sorts, len(sorts))
	for i, s := range sorts {
		cloned[i] = s.Clone()
	}
	return cloned
}

// SortNullsPosition is where NULLs sort (first or last).
type SortNullsPosition int8

const (
	// NullsPositionNotSpecified leaves NULLs position out of the query.
	NullsPositionNotSpecified SortNullsPosition = iota

	// NullsPositionFirst places NULLs first.
	NullsPositionFirst

	// NullsPositionLast places NULLs last.
	NullsPositionLast
)

// String returns a debug label ("nulls first"/"nulls last"/"").
func (s SortNullsPosition) String() string {
	switch s {
	case NullsPositionFirst:
		return "nulls first"
	case NullsPositionLast:
		return "nulls last"
	case NullsPositionNotSpecified:
		return ""
	default:
		return ""
	}
}

// SortDirection is a sort direction.
type SortDirection int8

const (
	// SortDirectionUnspecified falls back to the default direction.
	SortDirectionUnspecified SortDirection = iota
	// SortDirectionAsc sorts ascending.
	SortDirectionAsc
	// SortDirectionDesc sorts descending.
	SortDirectionDesc
)

// String returns a debug label ("asc"/"desc"/unspecified).
func (s SortDirection) String() string {
	switch s {
	case SortDirectionAsc:
		return "asc"
	case SortDirectionDesc:
		return "desc"
	case SortDirectionUnspecified:
		return strUnspecified
	default:
		return strUnspecified
	}
}
