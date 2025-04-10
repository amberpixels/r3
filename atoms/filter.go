package r3atoms

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Filter stands for generic Filter condition interface.
// For now, we have 2 implementations:
//  1. ColumnFilter - a single column from DB. Column Filter might be nested (children with AND/OR logic).
//     e.g. ColumFilter{Status = Active}
//  2. RawFilter - a raw Dialect (SQL) filter that can perform ANY given SQL query.
//     e.g. RawFilter{"WHERE EXISTS (SELECT 1 FROM ...."}
type Filter interface {
	// Stringer is needed for debugging purposes, so each filter can be printed.
	fmt.Stringer

	// DialectString returns the raw and SAFE dialect string (currently SQL only)
	// Second returning argument returns list of the arguments in case if first string has placeholders.
	DialectString() (string, []any)

	// ExtractJoins returns the list of joins (entity names) that are required to perform current filter.
	ExtractJoins() []string
}

// Filters represents a list of Filters.
// It is intentionally a slice of interface, so any filter can be inside.
type Filters []Filter

// RawFilter is a raw SQL filter that can perform ANY given SQL query.
// RawFilter can't have children (it's always a final leaf).
type RawFilter struct {
	rawSafeSQL string
	args       []any
	joins      []string
}

// JoinedOn is a simple setter on joins field.
func (f *RawFilter) JoinedOn(joins ...string) *RawFilter { f.joins = joins; return f }

// NewRawFilter creates a new RawFilter instance.
func NewRawFilter(rawSafeSQL string, args ...any) *RawFilter {
	return &RawFilter{rawSafeSQL: rawSafeSQL, args: args}
}

// String returns the rawSafeSQL and args sepaerately in one string.
// In a very simple manner.
func (f *RawFilter) String() string {
	return "SQL= " + f.rawSafeSQL + " ARGS=" + fmt.Sprintf("%v", f.args)
}

// DialectString returns the raw and SAFE dialect (SQL) string
func (f *RawFilter) DialectString() (string, []any) { return f.rawSafeSQL, f.args }

// ExtractJoins returns the list of joins specified.
func (f *RawFilter) ExtractJoins() []string { return f.joins }

// Ensure RawFilter always implements Filter interface.
var _ Filter = (*RawFilter)(nil)

// ColumnFilter represents a filtering criteria with a field, an operator, and a value.
type ColumnFilter struct {
	Field    string         `json:"f,omitempty"`
	Operator FilterOperator `json:"op,omitempty"`
	Value    any            `json:"v,omitempty"`

	// Children groups:
	// Note: When using AND/OR the Field,Operator,Value fields of the parent filter are ignored.

	// AND Children should be declared inside AND
	And Filters `json:"and,omitempty"`
	// OR Children should be declared inside OR
	Or Filters `json:"or,omitempty"`
}

// String returns safe sql (for now). TODO: it should be returned already executed
func (f *ColumnFilter) String() string { str, _ := f.DialectString(); return str }

// DialectString returns the Dialect (Safe SQL-ready) condition string with parameter placeholders,
// and a slice of arguments corresponding to each placeholder.
func (f *ColumnFilter) DialectString() (string, []any) {
	// Process the simple filter portion (if f.Field is not empty).
	if f.Field != "" {
		if f.Value == nil {
			switch f.Operator {
			case OperatorEq:
				return fmt.Sprintf("%s IS NULL", f.Field), []any{}
			case OperatorNe:
				return fmt.Sprintf("%s IS NOT NULL", f.Field), []any{}
			default:
				panic("should not happen. Somebody forgot to call Validate() on Filters")
			}
		}

		return fmt.Sprintf("%s %s ?", f.Field, f.Operator.DialectString()), []any{f.Value}
	}

	// Process AND / OR groups recursively.
	// Only one group can be present at a time.
	children, logicalOp := f.And, "AND"
	if len(f.Or) > 0 {
		children, logicalOp = f.Or, "OR"
	}

	var conditions []string
	var args []any
	for _, child := range children {
		cond, childArgs := child.DialectString()
		conditions = append(conditions, cond)
		args = append(args, childArgs...)
	}

	return "(" + strings.Join(conditions, " "+logicalOp+" ") + ")", args
}

// ExtractJoins extracts joins from the filter (by prefixes of the field names)
func (f *ColumnFilter) ExtractJoins() []string {
	// Check the current filter's Field.
	if f.Field != "" {
		if join := extractJoinFromField(f.Field); join != "" {
			return []string{join}
		}
		return []string{}
	}

	// Process AND/OR.
	// Only one group must be non-empty.
	group := f.And
	if len(f.Or) > 0 {
		group = f.Or
	}

	joins := make([]string, 0)
	for _, child := range group {
		for _, join := range child.ExtractJoins() {
			if slices.Contains(joins, join) {
				continue
			}
			joins = append(joins, join)
		}
	}

	return joins
}

//
// Filters as collection of any filters
//

// DialectString returns the Dialect (Safe SQL-Ready) string and args for a list of filters.
// It is considered to be called only for the root filters list, as otherwise deep filters list are handled
// via parent's Filter.DialectString() recursively.
//
// For root Filters list we by default use the AND logic.
func (fs Filters) DialectString() (string, []any) {
	var conditions []string
	var args []any
	for _, f := range fs {
		cond, childArgs := f.DialectString()
		conditions = append(conditions, cond)
		args = append(args, childArgs...)
	}
	return strings.Join(conditions, " AND "), args
}

// ExtractJoins recursively extracts join table names from all filters.
// A join is detected if a filter's Field starts with a quoted entity name,
// e.g. `"City".name` will extract the join "City".
// The returned list is de-duplicated.
func (fs Filters) ExtractJoins() []string {
	joins := make([]string, 0)

	for _, f := range fs {
		for _, join := range f.ExtractJoins() {
			if slices.Contains(joins, join) {
				continue
			}
			joins = append(joins, join)
		}
	}

	return joins
}

//
// Auxiliary helpers for quick filter scaffolding:
//

// NewColumnFilter is a simple constructor for the ColumnFilter (not an AND/OR group).
func NewColumnFilter(field string, operator FilterOperator, value any) *ColumnFilter {
	return &ColumnFilter{Field: field, Operator: operator, Value: value}
}

// Fop is a shorthand for NewFilter()
// Fop is "F" for filter and "op" for operator.
func Fop(field string, operator FilterOperator, value any) *ColumnFilter {
	return NewColumnFilter(field, operator, value)
}

// F is a shorthand for NewColumnFilter(field, OperatorEq, value)
func F(field string, value any) *ColumnFilter      { return Fop(field, OperatorEq, value) }
func FLike(field string, value any) *ColumnFilter  { return Fop(field, OperatorLike, value) }
func FILike(field string, value any) *ColumnFilter { return Fop(field, OperatorILike, value) }

// And is a constructor of AND group of filters.
func And(filters ...Filter) *ColumnFilter { return &ColumnFilter{And: filters} }

// Or is a constructor of OR group of filters.
func Or(filters ...Filter) *ColumnFilter { return &ColumnFilter{Or: filters} }

// ParseFilters parses a given raw JSON string into a slice of filters.
func ParseFilters(filtersJsonRaw string) (Filters, error) {
	// Only ColumnFilter are for now parseable
	var columnFilters []*ColumnFilter
	if err := json.Unmarshal([]byte(filtersJsonRaw), &columnFilters); err != nil {
		var singleFilter ColumnFilter
		if singleFilterErr := json.Unmarshal([]byte(filtersJsonRaw), &singleFilter); singleFilterErr == nil {
			return Filters{&singleFilter}, nil
		}

		return nil, fmt.Errorf("invalid columnFilters: %s", filtersJsonRaw)
	}

	filters := make(Filters, len(columnFilters))
	for i, filter := range columnFilters {
		filters[i] = filter
	}

	return filters, nil
}

// extractJoinFromField extracts a join from a given field name.
// It only returns a join if the field is explicitly quoted.
// E.g. for `"City".Status` it returns "City", otherwise it returns an empty string.
func extractJoinFromField(field string) string {
	// Only consider fields that start with a double quote.
	if len(field) > 0 && field[0] == '"' {
		// Find the closing double quote.
		// Note: We assume valid syntax (i.e. there is a matching quote).
		endQuoteIndex := strings.Index(field[1:], `"`)
		if endQuoteIndex != -1 {
			// Extract the quoted join name.
			join := field[1 : endQuoteIndex+1]
			return join
		}
	}
	return ""
}
