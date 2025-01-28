package crood

// FilterOperator defines the type for filter operations.
type FilterOperator string

// Predefined filter operators.
const (
	OperatorEquals         FilterOperator = "="
	OperatorNotEquals      FilterOperator = "!="
	OperatorGreaterThan    FilterOperator = ">"
	OperatorLessThan       FilterOperator = "<"
	OperatorGreaterOrEqual FilterOperator = ">="
	OperatorLessOrEqual    FilterOperator = "<="
	OperatorLike           FilterOperator = "LIKE"
	OperatorIn             FilterOperator = "IN"
	OperatorNotIn          FilterOperator = "NOT IN"
)

func (fo FilterOperator) String() string {
	return string(fo)
}

// Filter defines a single filtering rule.
type Filter interface {
	GetField() Field             // The field to filter by, e.g., "status"
	GetOperator() FilterOperator // The comparison operator, e.g., "=", ">", "<", "LIKE"
	GetValue() any               // The value to filter against, e.g., "active", 100

	// TODO: Handle nested filters e.g. grouping via AND/OR/NOT/NOR/etc
}

// FilterList is a slice of Filter.
type FilterList []Filter
