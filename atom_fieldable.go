package crood

// Fieldable defines a single field selection rule.
type Fieldable interface {
	// String returns the field name stringified
	String() string // Field name, e.g., "id" or "name"

	// TODO: Nested subfields e.g., "address.city"
	// GetSubfields() Fieldables
}

// Fieldables is an interface for a collection of fieldables.
type Fieldables interface {
	Fields() []Fieldable
	Strings() []string
}

type Field struct {
	Name string
}

func (f Field) String() string { return f.Name }

type Fields []Field

func (fs Fields) List() []Fieldable {
	fields := make([]Fieldable, len(fs))
	for i, f := range fs {
		fields[i] = f
	}
	return fields
}

func (fs Fields) Strings() []string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.String()
	}
	return names
}

type StringField string

func (f StringField) String() string { return string(f) }

type StringFields []StringField

func (fs StringFields) Fields() []Fieldable {
	fields := make([]Fieldable, len(fs))
	for i, f := range fs {
		fields[i] = f
	}
	return fields
}

func (fs StringFields) Strings() []string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.String()
	}
	return names
}
