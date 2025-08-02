package r3

import (
	"errors"
	"fmt"
)

// TODO(future): Thought for future imporovements:
// Field may be a more complex thing:
// We should distinguish between object field and leaf field (e.g. card.image vs card.image.url)
// (very optional) We should support computed field e.g. (card.image.url(thumbnail=X)))

// Field defines a single field selection rule.
type Field interface {
	// Stringer is needed for debugging purposes, so each field can be printed.
	fmt.Stringer

	// Cloner is needed so we fields are safe and immutable
	Cloner[Field]

	// ToDialect converts the Field (r3=>SQL) into its dialect-specific representation.
	ToDialect(FieldOutboundDialector) (DialectValue, error)

	// FromDialect makes up the Field (JSON=>r3) from an FieldInboundDialector and its DialectValue
	FromDialect(FieldInboundDialector, DialectValue) error
}

// Fields is a slice of Field-s.
type Fields []Field

type (
	// FieldOutboundDialector is a generic dialector (visitor) interface for conversion.
	FieldOutboundDialector interface {
		TranslateFieldSpec(cf *FieldSpec) (DialectValue, error)
	}

	// FieldInboundDialector is a generic inbound dialector (visitor) interface for conversion.
	FieldInboundDialector interface {
		TranslateIntoFieldSpec(f DialectValue) (*FieldSpec, error)
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

// Clone returns a safe full-clone of the fields list.
func (fs Fields) Clone() Fields { return cloneAll(fs) }

// FieldSpec is the simplest possible implementation of the Field interface.
// FieldSpec is just a string - it can be the name of the field in database, etc.
type FieldSpec string

func NewFieldSpec(s string) *FieldSpec {
	var cf = new(FieldSpec)
	*cf = FieldSpec(s)
	return cf
}

var _ Field = (*FieldSpec)(nil)

// String simply returns its value.
func (f *FieldSpec) String() string {
	if f == nil {
		return ""
	}
	return string(*f)
}

// Clone returns a clone of the field.
func (f *FieldSpec) Clone() Field {
	clone := *f
	return &clone
}

// ToDialect converts the Field into its dialect-specific representation.
func (f *FieldSpec) ToDialect(dialector FieldOutboundDialector) (DialectValue, error) {
	return dialector.TranslateFieldSpec(f)
}

func (f *FieldSpec) FromDialect(dialector FieldInboundDialector, inValue DialectValue) error {
	if f == nil {
		return errors.New("FromDialect must be called on a non-nil ColumnField")
	}

	dialected, err := dialector.TranslateIntoFieldSpec(inValue)
	if err != nil {
		return fmt.Errorf("inbound dialector failed: %w", err)
	}

	*f = *dialected
	return nil
}
