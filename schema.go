package r3

// Schema is the logical, engine-agnostic descriptor of an entity: an ordered set
// of capability-bearing Attributes plus lookup helpers. It lives in the core r3
// package next to Query/Filters and contains no storage details (no SQL column
// names, no BSON paths) — each engine owns the physical binding, keyed by
// attribute name.
//
// A Schema is immutable: derive one with SchemaOf and layer overrides with With,
// both of which return a fresh Schema. The zero Schema has no attributes;
// ValidateQuery treats it as "no schema" and validates nothing, which keeps the
// engine seam back-compatible for callers that pass no schema.
//
// Vocabulary note: an Attribute is a declared member of an entity, whereas a
// Field/FieldSpec is a reference to one inside a Query. The Schema is the set of
// valid Fields, plus what each is allowed to do.
type Schema struct {
	attrs []Attribute
	index map[string]int // attribute name -> position in attrs
}

// Attribute is one declared, capability-bearing member of an entity.
type Attribute struct {
	// Name is the public/wire name, e.g. "created_at" (snake_case). Filters,
	// sorts, and selected fields reference attributes by this name; the engine
	// translates it to a physical column/path.
	Name string

	// Type is the logical data type, used to pick default operators and drive
	// frontend filter widgets.
	Type DataType

	// Caps is the bitset of capabilities (Filterable, Sortable, ...).
	Caps Capability

	// Ops are the filter operators allowed for this attribute. Derivation fills
	// it from the Type's defaults when the attribute is filterable; nil means
	// "the defaults for Type" (or none, when not filterable).
	Ops []FilterOperatorSpec

	// Enum holds the allowed values when Type == TypeEnum.
	Enum []string

	// Relation describes the target when Type == TypeRel.
	Relation *RelationRef

	// Computed marks an attribute with no backing column (reserved; computed
	// execution is out of scope — see the schema design doc, §8). A computed
	// attribute can never be written: the structural floor has nowhere to put a
	// value, so no escape hatch can corrupt it.
	Computed bool
}

// RelationRef points an Attribute (Type == TypeRel) at its target entity.
type RelationRef struct {
	// Target is the related entity's logical name (snake_case of its type).
	Target string
	// Kind is the relationship kind ("has-many", "belongs-to", "many-to-many").
	Kind string
	// Label is the attribute on the target used as a human-facing label
	// (optional; populated via With for introspection).
	Label string
}

// Has reports whether the attribute carries every capability in c.
func (a Attribute) Has(c Capability) bool { return a.Caps&c == c }

// newSchema builds a Schema from an ordered attribute slice, indexing by name.
// On a duplicate name the last attribute wins (matches With's override
// semantics); the slice order is preserved for stable introspection.
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

// Attributes returns the schema's attributes in declaration order. The returned
// slice is a copy; mutating it does not affect the Schema.
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

// With returns a new Schema with the given attributes layered on top: an
// override whose Name matches an existing attribute replaces it in place;
// a new Name is appended. The receiver is left unchanged.
//
// Use it for what tags cannot express — computed attributes, relation labels,
// tightened operator sets — keeping the Schema immutable.
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
