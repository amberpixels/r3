package r3atoms

import (
	"fmt"
)

// Field defines a single field selection rule.
type Field interface {
	fmt.Stringer

	DialectString() string

	// TODO: Nested subfields e.g., "address.city"
	// GetSubfields() Fields
}

type Fields []Field

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

func (f ColumnField) DialectString() string { return string(f) }

// TODO(?) More complex way of storing a field
// It can be something like
// struct Field struct {Name string  ... params}
// So we can add more fields, params to the field when needed
