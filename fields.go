package r3

import (
	"fmt"
)

// Field defines a single field selection rule.
type Field interface {
	fmt.Stringer

	// ToDialect converts the Field into its dialect-specific representation.
	ToDialect(FieldDialector) (DialectValue, error)

	// TODO: Nested subfields e.g., "address.city"
	// GetSubfields() Fields
}

// Fields is a slice of Field-s.
type Fields []Field

type (
	// FieldDialector is a generic dialector (visitor) interface for conversion.
	FieldDialector interface {
		FromColumnField(cf ColumnField) (DialectValue, error)
	}

	// FieldInboundDialector is a generic inbound dialector (visitor) interface for conversion.
	FieldInboundDialector interface {
		ToField(f DialectValue) (Field, error)
	}
)

// MergeWith merges (combines) fields with other fields ()
func (fs Fields) MergeWith(other Fields) Fields { return mergeWith(fs, other) }

// Dedupe removes duplicates from the fields list
// Not: it's not super-performant because of types and go-generics. Refactor if needed
func (fs *Fields) Dedupe() {
	v := []Field(*fs)
	dedupe[Field](&v)
	*fs = Fields(v)
}

// ColumnField is the simplest possible implementation of Fieldable
// Here, we just mean Field for a column in the database.
type ColumnField string

// String simply returns its value
func (f ColumnField) String() string { return string(f) }

// ToDialect converts the Field into its dialect-specific representation.
func (f ColumnField) ToDialect(dialector FieldDialector) (DialectValue, error) {
	return dialector.FromColumnField(f)
}

// TODO(?) More complex way of storing a field
// It can be something like
// struct Field struct {Name string  ... params}
// So we can add more fields, params to the field when needed
