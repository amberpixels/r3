package depo

// Fieldable defines a single field selection rule.
type Fieldable interface {
	// String returns the field name stringified, e.g. "id", "name", etc
	String() string

	// TODO: Nested subfields e.g., "address.city"
	// GetSubfields() Fieldables
}

// Fieldables is an interface for a collection of fieldables.
type Fieldables interface {
	// Fields returns a slice of Fieldable's
	Fields() []Fieldable

	// Strings is a shortcut for calling each Fields().String()
	Strings() []string
}

// StringField is the simplest possible implementation of Fieldable
type StringField string

// String simply returns its value
func (f StringField) String() string { return string(f) }

// StringFields is a list of string fields
type StringFields []StringField

// Fields returns the string fields as []Fieldable
func (fs StringFields) Fields() []Fieldable {
	fields := make([]Fieldable, len(fs))
	for i, f := range fs {
		fields[i] = f
	}
	return fields
}

// Strings returns string fields as []string
func (fs StringFields) Strings() []string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.String()
	}
	return names
}

// TODO(?) More complex way of storing a field
// It can be something like
// struct Field struct {Name string  ... params}
// So we can add more fields, params to the field when needed
