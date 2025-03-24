package r3

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
	Len() int
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

type FiltersGroup struct {
	filters Filters
}

func NewFiltersGroup() *FiltersGroup {
	return &FiltersGroup{}
}

var _ Filterables = &FiltersGroup{}

func (fg *FiltersGroup) Where(field Fieldable, operator FilterOperator, value any) *FiltersGroup {
	fg.filters = append(fg.filters, Filter{field, operator, value})
	return fg
}
func (fg *FiltersGroup) WhereTrue(fieldName string) *FiltersGroup {
	return fg.Where(StringField(fieldName), OperatorEquals, true)
}
func (fg *FiltersGroup) WhereEq(fieldName string, v any) *FiltersGroup {
	return fg.Where(StringField(fieldName), OperatorEquals, v)
}

func (fg *FiltersGroup) Append(f Filter) *FiltersGroup {
	fg.filters = append(fg.filters, f)
	return fg
}
func (fb *FiltersGroup) GetFilters() []Filterable {
	result := make([]Filterable, 0)
	for _, f := range fb.filters {
		result = append(result, f)
	}
	return result
}
func (fb *FiltersGroup) Len() int { return len(fb.filters) }
