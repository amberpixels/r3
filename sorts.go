package r3

import (
	"errors"
	"fmt"
	"strings"
)

// Sort defines the sort criteria and direction.
type Sort interface {
	fmt.Stringer

	// ToDialect converts the Sort (r3=>SQL) into its dialect-specific representation.
	ToDialect(SortOutboundDialector) (DialectValue, error)

	// FromDialect makes up the Sort (JSON=>r3) from a SortInboundDialector and its DialectValue
	FromDialect(SortInboundDialector, DialectValue) error

	GetDirection() SortDirection
	GetCriteria() string
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
	Column    FieldSpec
	Direction SortDirection // Asc | Desc | Unspecified (default)

	NullsPosition SortNullsPosition // First | Last | Unspecified
}

// ToDialect converts the Sort into its dialect-specific representation.
func (s *SortSpec) ToDialect(dialector SortOutboundDialector) (DialectValue, error) {
	return dialector.TranslateSortSpec(s)
}

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

func (s *SortSpec) String() string {
	// For debugging - use a simple format
	str := s.Column.String() + " " + s.Direction.String()
	if s.NullsPosition != NullsPositionNotSpecified {
		str += " " + s.NullsPosition.String()
	}
	return str
}

func (s *SortSpec) GetDirection() SortDirection { return s.Direction }

func (s *SortSpec) GetCriteria() string { return s.Column.String() }

func NewSortSpec(col FieldSpec, direction SortDirection, nullsPositionArg ...SortNullsPosition) *SortSpec {
	s := &SortSpec{
		Column:    col,
		Direction: direction,
	}

	if len(nullsPositionArg) > 0 {
		s.NullsPosition = nullsPositionArg[0]
	}

	return s
}

var _ Sort = (*SortSpec)(nil)

// Sorts represents a list of ColumnSort-s.
type Sorts []Sort

func (sorts Sorts) MergeWith(other Sorts) Sorts { return mergeWith(sorts, other) }

// NewSorts parses the given raw sorting string and returns a list of Sorts.
// Currently, we do support curator's API handler's `field ASC NULLS FIRST` syntax.
// Later, we should reconsider the string syntax and it may be changed.
func NewSorts(sortingOrder ...string) (Sorts, error) {
	if len(sortingOrder) == 0 {
		return Sorts{}, nil
	}

	// edge case: when a single sortingOrder is given
	// it might be a "field1 ASC, field2 DESC" string, so let's split it
	if len(sortingOrder) == 1 {
		sortingOrder = strings.Split(sortingOrder[0], ",")
	}

	sorts := make(Sorts, 0, len(sortingOrder))

	for _, orderRawString := range sortingOrder {
		orderRawString = strings.TrimSpace(orderRawString)

		// if having no spaces means it's just a field name
		// use whole string as the field name then.
		if !strings.Contains(orderRawString, " ") {
			sorts = append(sorts, &SortSpec{Column: FieldSpec(orderRawString)})
			continue
		}

		orderWords := strings.Split(orderRawString, " ")

		col := FieldSpec(orderWords[0])
		direction := ParseSortDirection(orderWords[1])
		nullsPosition := NullsPositionNotSpecified

		if len(orderWords) > 2 {
			restWords := strings.Join(orderWords[2:], " ") // it's should be NULLS FIRST | NULLS LAST
			nullsPosition = ParseSortNullsPosition(restWords)

			// TODO: find out can we do blanket default, or it will be failing for fields that are not nullable?
		}

		sorts = append(sorts, &SortSpec{
			Column:        col,
			Direction:     direction,
			NullsPosition: nullsPosition,
		})
	}

	return sorts, nil
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

// DialectString returns a SQL-ready string representation of SortNullsPosition.
func (s SortNullsPosition) DialectString() string {
	// TODO: dialect to be respected
	switch s {
	case NullsPositionFirst:
		return "NULLS FIRST"
	case NullsPositionLast:
		return "NULLS LAST"
	case NullsPositionNotSpecified:
		fallthrough
	default:
		return ""
	}
}

// String is implemented for debugging purposes, so the SortNullsPosition is a fmt.Stringer.
func (s SortNullsPosition) String() string { return s.DialectString() }

// ParseSortNullsPosition parses the given string and returns a SortNullsPosition.
func ParseSortNullsPosition(s string) SortNullsPosition {
	switch strings.ToUpper(s) {
	case NullsPositionFirst.String():
		return NullsPositionFirst
	case NullsPositionLast.String():
		return NullsPositionLast
	default:
		return NullsPositionNotSpecified
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

// DialectString returns a Dialect (safe SQL-ready) string representation of SortDirection.
func (s SortDirection) DialectString() string {
	switch s {
	case SortDirectionAsc:
		return "ASC"
	case SortDirectionDesc:
		return "DESC"
	case SortDirectionUnspecified:
		fallthrough
	default:
		return "DESC" // as default
	}
}

// String is implemented for debugging purposes, so the SortDirection is a fmt.Stringer.
func (s SortDirection) String() string { return s.DialectString() }

// ParseSortDirection parses the given string and returns a SortDirection.
func ParseSortDirection(s string) SortDirection {
	switch strings.ToUpper(s) {
	case SortDirectionAsc.String():
		return SortDirectionAsc
	case SortDirectionDesc.String():
		return SortDirectionDesc
	default:
		return SortDirectionUnspecified
	}
}
