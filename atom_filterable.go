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

func (fo FilterOperator) String() string { return string(fo) }

// Filterable defines a single filtering rule.
type Filterable interface {
	GetField() Fieldable         // The field to filter by, e.g., "status"
	GetOperator() FilterOperator // The comparison operator, e.g., "=", ">", "<", "LIKE"
	GetValue() any               // The value to filter against, e.g., "active", 100

	// TODO: Handle nested filters e.g. grouping via AND/OR/NOT/NOR/etc
}

// Filterables is a collection of Filterable.
type Filterables interface {
	GetFilters() []Filterable
}

type Filter struct {
	Field    Fieldable
	Operator FilterOperator
	Value    any
}

func (f Filter) GetField() Fieldable         { return f.Field }
func (f Filter) GetOperator() FilterOperator { return f.Operator }
func (f Filter) GetValue() any               { return f.Value }

// Filters is a slice of Filter.
type Filters []Filter

type FiltersBuilder struct {
	filters Filters
}

func NewFiltersBuilder() *FiltersBuilder {
	return &FiltersBuilder{}
}

func (fb *FiltersBuilder) Add(field Fieldable, operator FilterOperator, value any) *FiltersBuilder {
	fb.filters = append(fb.filters, Filter{field, operator, value})
	return fb
}

func (fb *FiltersBuilder) Build() Filters {
	return fb.filters
}
