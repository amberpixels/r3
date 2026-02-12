package r3

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
