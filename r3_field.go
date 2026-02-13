package r3

import (
	"errors"
	"strings"
)

// ErrInvalidIdentifier is returned when a field name contains characters
// that are not valid SQL identifiers.
var ErrInvalidIdentifier = errors.New("invalid identifier")

// ValidateIdentifier checks that s is a safe SQL identifier or dotted identifier path.
// Each dot-separated segment must match [a-zA-Z_][a-zA-Z0-9_]*.
// Examples of valid identifiers: "id", "user_name", "user.profile", "orders.items.product_name".
// Examples of invalid identifiers: "", "1col", "a b", "x;y", "col--", "table.*".
func ValidateIdentifier(s string) error {
	if s == "" {
		return ErrInvalidIdentifier
	}

	segments := strings.Split(s, ".")
	for _, seg := range segments {
		if !isValidIdentifierSegment(seg) {
			return ErrInvalidIdentifier
		}
	}
	return nil
}

// isValidIdentifierSegment checks that a single identifier segment (no dots)
// matches [a-zA-Z_][a-zA-Z0-9_]*.
func isValidIdentifierSegment(s string) bool {
	if len(s) == 0 {
		return false
	}

	first := s[0]
	if !isLetter(first) && first != '_' {
		return false
	}

	for i := 1; i < len(s); i++ {
		c := s[i]
		if !isLetter(c) && !isDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

// TODO(future): Thought for future improvements:
// Field may be a more complex thing:
// We should distinguish between object field and leaf field (e.g. card.image vs card.image.url)
// (very optional) We should support computed field e.g. (card.image.url(thumbnail=X)))

// Fields is a slice of *FieldSpec.
type Fields []*FieldSpec

// MergeWith merges (combines) fields with other fields.
func (fs Fields) MergeWith(other Fields) Fields { return mergeWith(fs, other) }

// Dedupe removes duplicates from the fields list.
func (fs *Fields) Dedupe() {
	v := []*FieldSpec(*fs)
	dedupe(&v)
	*fs = v
}

// Clone returns a safe full-clone of the fields list.
func (fs Fields) Clone() Fields {
	cloned := make(Fields, len(fs))
	for i, f := range fs {
		cloned[i] = f.Clone()
	}
	return cloned
}

// FieldSpec is the simplest possible implementation of a field.
// FieldSpec is just a string - it can be the name of the field in database, etc.
type FieldSpec string

func NewFieldSpec(s string) *FieldSpec {
	var cf = new(FieldSpec)
	*cf = FieldSpec(s)
	return cf
}

// String simply returns its value.
func (f *FieldSpec) String() string {
	if f == nil {
		return ""
	}
	return string(*f)
}

// Clone returns a clone of the field.
func (f *FieldSpec) Clone() *FieldSpec {
	clone := *f
	return &clone
}
