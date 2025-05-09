package r3

import (
	"fmt"
)

// TODO(?): Thought for future improvements:
// Field may be a more complex thing:
// We should distinguish between object field and leaf field (e.g. card.image vs card.image.url)
// (very optional) We should support computed field e.g. (card.image.url(thumbnail=X)))

// Field defines a single field selection rule.
type Field interface {
	fmt.Stringer

	// ToDialect converts the Field into its dialect-specific representation.
	// E.g. r3 => SQL
	ToDialect(FieldOutboundDialector) (DialectValue, error)

	// FromDialect makes up a field from an FieldInboundDialector and its DialectValue
	// e.g. JSON => r3
	FromDialect(FieldInboundDialector, DialectValue) error
}

// Fields is a slice of Field-s.
type Fields []Field

type (
	// FieldOutboundDialector is a generic dialector (visitor) interface for conversion.
	FieldOutboundDialector interface {
		TranslateColumnField(cf *ColumnField) (DialectValue, error)
	}

	// FieldInboundDialector is a generic inbound dialector (visitor) interface for conversion.
	FieldInboundDialector interface {
		TranslateIntoColumnField(f DialectValue) (*ColumnField, error)
	}
)

// MergeWith merges (combines) fields with other fields ().
func (fs Fields) MergeWith(other Fields) Fields { return mergeWith(fs, other) }

// Dedupe removes duplicates from the fields list.
// Note: it's not super-performant because of types and go-generics. Refactor if needed.
func (fs *Fields) Dedupe() {
	v := []Field(*fs)
	dedupe(&v)
	*fs = v
}

// ColumnField is the simplest possible implementation of Fieldable
// Here, we just mean Field for a column in the database.
type ColumnField string

func NewColumnField(s string) *ColumnField {
	var cf = new(ColumnField)
	*cf = ColumnField(s)
	return cf
}

var _ Field = (*ColumnField)(nil)

// String simply returns its value.
func (f *ColumnField) String() string { return string(*f) }

// ToDialect converts the Field into its dialect-specific representation.
func (f *ColumnField) ToDialect(dialector FieldOutboundDialector) (DialectValue, error) {
	return dialector.TranslateColumnField(f)
}

func (f *ColumnField) FromDialect(dialector FieldInboundDialector, inValue DialectValue) error {
	if f == nil {
		return fmt.Errorf("FromDialect must be called on a non-nil ColumnField")
	}

	translated, err := dialector.TranslateIntoColumnField(inValue)
	if err != nil {
		return fmt.Errorf("inbound dialector fialed: %w", err)
	}

	*f = *translated
	return nil
}
