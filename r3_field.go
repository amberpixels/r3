package r3

import (
	"errors"
	"strings"
)

// ErrInvalidIdentifier is returned when a field name contains characters
// that are not valid SQL identifiers.
var ErrInvalidIdentifier = errors.New("invalid identifier")

// ErrNoPatchFields is returned by Patch when the Fields list is empty or nil.
var ErrNoPatchFields = errors.New("patch requires at least one field")

// ErrInvalidPatchField is returned by a write (Patch, or full Update SET-shaping)
// when a field name does not match any attribute in the schema, or names an
// attribute that is not mutable (e.g. PK, created_at, soft-delete, immutable).
var ErrInvalidPatchField = errors.New("invalid patch field")

// ValidateIdentifier checks that s is a safe SQL identifier or dotted path; each
// dot-separated segment must match [a-zA-Z_][a-zA-Z0-9_]*. Valid: "id",
// "user.profile", "orders.items.product_name". Invalid: "1col", "a b", "x;y",
// "table.*".
func ValidateIdentifier(s string) error {
	if s == "" {
		return ErrInvalidIdentifier
	}

	for seg := range strings.SplitSeq(s, ".") {
		if !isValidIdentifierSegment(seg) {
			return ErrInvalidIdentifier
		}
	}
	return nil
}

// isValidIdentifierSegment checks that a single dot-free segment matches
// [a-zA-Z_][a-zA-Z0-9_]*.
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

// FieldSpec names a field: a plain string (a column/attribute name, possibly a
// dotted path).
type FieldSpec string

// NewFieldSpec returns a *FieldSpec for s.
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

// FieldsToStrings converts Fields to a []string of field names (nils skipped).
func FieldsToStrings(fields Fields) []string {
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != nil {
			out = append(out, f.String())
		}
	}
	return out
}
