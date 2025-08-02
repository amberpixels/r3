package r3

import (
	"errors"
	"fmt"
)

// Sort defines the sort criteria and direction.
type Sort interface {
	// Stringer is needed for debugging purposes, so each sort can be printed.
	fmt.Stringer

	// Cloner is needed so we sorts are safe and immutable
	Cloner[Sort]

	// ToDialect converts the Sort (r3=>SQL) into its dialect-specific representation.
	ToDialect(SortOutboundDialector) (DialectValue, error)

	// FromDialect makes up the Sort (JSON=>r3) from a SortInboundDialector and its DialectValue
	FromDialect(SortInboundDialector, DialectValue) error

	// GetDirection returns the direction of the sort
	GetDirection() SortDirection

	// GetCriteria returns the criteria (e.g. Field, on which we do sort)
	GetCriteria() Field
}

type (
	// SortOutboundDialector is a generic dialector (visitor) interface for conversion.
	SortOutboundDialector interface {
		TranslateSortSpec(s *SortSpec) (DialectValue, error)
	}

	// SortInboundDialector is a generic inbound dialector (visitor) interface for conversion.
	SortInboundDialector interface {
		TranslateIntoSortSpec(s DialectValue) (*SortSpec, error)
	}
)

// SortSpec represents a single sort criteria.
type SortSpec struct {
	Column    *FieldSpec
	Direction SortDirection // Asc | Desc | Unspecified (default)

	NullsPosition SortNullsPosition // First | Last | Unspecified
}

// ToDialect converts the Sort into its dialect-specific representation.
func (s *SortSpec) ToDialect(dialector SortOutboundDialector) (DialectValue, error) {
	return dialector.TranslateSortSpec(s)
}

// FromDialect converts a dialect-specific sort representation into r3 Sort.
func (s *SortSpec) FromDialect(dialector SortInboundDialector, inValue DialectValue) error {
	if s == nil {
		return errors.New("FromDialect must be called on a non-nil SortSpec")
	}

	translated, err := dialector.TranslateIntoSortSpec(inValue)
	if err != nil {
		return fmt.Errorf("inbound dialector failed: %w", err)
	}

	*s = *translated
	return nil
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
func (s *SortSpec) GetCriteria() Field { return s.Column }

// Clone returns a new pointer to the sort spec (safe and deep clone).
func (s *SortSpec) Clone() Sort {
	var clone SortSpec
	clone.Column, _ = s.Column.Clone().(*FieldSpec)
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

var _ Sort = (*SortSpec)(nil)

// Sorts represents a list of ColumnSort-s.
type Sorts []Sort

// MergeWith merges other sorts into ours.
// TODO: here and in other places, MergeWith must be safe? immutable, right?? think and plan
func (sorts Sorts) MergeWith(other Sorts) Sorts { return mergeWith(sorts, other) }

// Clone returns a cloned list of given sorts.
func (sorts Sorts) Clone() Sorts {
	clone := make(Sorts, len(sorts))
	for i, s := range sorts {
		clone[i] = s.Clone()
	}

	return clone
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
		return "unspecified"
	}
}
