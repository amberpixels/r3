package r3

// Schema is the logical, engine-agnostic descriptor of an entity: an ordered set
// of capability-bearing [Attribute]s. It carries no storage details (no column
// names, no BSON paths) - each engine owns the physical binding, keyed by
// attribute name.
//
// A Schema is immutable: derive with [SchemaOf], layer overrides with
// [Schema.With], both returning a fresh value. The zero Schema has no attributes
// and validates nothing (back-compat for schema-less callers); see [Schema.IsZero].
//
// Vocabulary: an Attribute is a declared member of an entity; a [FieldSpec] is a
// reference to one inside a [Query]. The Schema is the set of valid fields plus
// what each may do.
type Schema struct {
	attrs []Attribute
	index map[string]int // attribute name -> position in attrs
}

// Attribute is one declared, capability-bearing member of an entity.
type Attribute struct {
	// Name is the public/wire name (snake_case). Queries reference attributes by
	// this name; the engine translates it to a physical column/path.
	Name string

	// Type is the logical data type, used to pick default operators and drive
	// frontend filter widgets.
	Type DataType

	// Caps is the bitset of capabilities.
	Caps Capability

	// Ops are the allowed filter operators. nil means "the defaults for Type"
	// (or none, when not filterable).
	Ops []FilterOperatorSpec

	// Enum holds the allowed values when Type == TypeEnum.
	Enum []string

	// Relation describes the target when Type == TypeRel.
	Relation *RelationRef

	// Codec, when non-nil, transforms this attribute's value to and from its
	// stored form (see [Codec]). Type stays the domain type (e.g. TypeTime) so
	// callers filter with domain values; the codec bridges to storage
	// ([Codec.Stored], e.g. TypeInt) at the engine boundary. nil is the identity.
	Codec Codec

	// Computed marks an attribute with no backing column (reserved; computed
	// execution is out of scope - see the schema design doc, §8). A computed
	// attribute can never be written: the structural floor has nowhere to put a
	// value.
	Computed bool
}

// RelationRef points an Attribute (Type == TypeRel) at its target entity.
type RelationRef struct {
	// Target is the related entity's logical name (snake_case of its type).
	Target string
	// Kind is the relationship kind ("has-many", "belongs-to", "many-to-many").
	Kind string
	// Label is the target attribute used as a human-facing label (optional;
	// populated via With for introspection).
	Label string
}

// Has reports whether the attribute carries every capability in c.
func (a Attribute) Has(c Capability) bool { return a.Caps&c == c }

// newSchema builds a Schema from an ordered attribute slice, indexing by name.
// On a duplicate name the last wins (matches With); slice order is preserved.
func newSchema(attrs []Attribute) Schema {
	index := make(map[string]int, len(attrs))
	for i, a := range attrs {
		index[a.Name] = i
	}
	return Schema{attrs: attrs, index: index}
}

// IsZero reports whether the schema declares no attributes. A zero Schema
// disables validation at the engine seam (back-compat for schema-less callers).
func (s Schema) IsZero() bool { return len(s.attrs) == 0 }

// Attributes returns the attributes in declaration order (a copy).
func (s Schema) Attributes() []Attribute {
	return append([]Attribute(nil), s.attrs...)
}

// Lookup returns the attribute with the given name and whether it exists.
func (s Schema) Lookup(name string) (Attribute, bool) {
	if i, ok := s.index[name]; ok {
		return s.attrs[i], true
	}
	return Attribute{}, false
}

// With returns a new Schema with the overrides layered on: a matching Name
// replaces in place, a new Name is appended; the receiver is unchanged. Use it
// for what tags cannot express - computed attributes, relation labels,
// tightened operator sets.
func (s Schema) With(overrides ...Attribute) Schema {
	merged := append([]Attribute(nil), s.attrs...)
	for _, a := range overrides {
		if i, ok := s.index[a.Name]; ok {
			merged[i] = a
		} else {
			merged = append(merged, a)
		}
	}
	return newSchema(merged)
}
