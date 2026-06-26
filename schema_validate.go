package r3

import (
	"fmt"
	"strings"
)

// WriteOp identifies which write a capability check applies to.
type WriteOp uint8

const (
	// WriteOpCreate is a Create — gated by the Creatable capability.
	WriteOpCreate WriteOp = iota
	// WriteOpMutate is an Update or Patch — gated by the Mutable capability.
	WriteOpMutate
)

// Filterable reports whether the named attribute may appear in Query.Filters.
func (s Schema) Filterable(name string) bool { return s.hasCap(name, Filterable) }

// Sortable reports whether the named attribute may appear in Query.Sorts.
func (s Schema) Sortable(name string) bool { return s.hasCap(name, Sortable) }

// Queryable reports whether the named attribute may appear in Query.Fields.
func (s Schema) Queryable(name string) bool { return s.hasCap(name, Queryable) }

// Writable reports whether the named attribute may be written by the given op.
func (s Schema) Writable(name string, op WriteOp) bool {
	a, ok := s.Lookup(name)
	if !ok {
		return false
	}
	switch op {
	case WriteOpCreate:
		return a.Has(Creatable)
	case WriteOpMutate:
		return a.Has(Mutable)
	default:
		return false
	}
}

// hasCap is the shared lookup-then-check used by the per-capability helpers.
func (s Schema) hasCap(name string, c Capability) bool {
	a, ok := s.Lookup(name)
	return ok && a.Has(c)
}

// ValidateQuery is the source of typed, 400-class errors. It checks every field
// referenced in Filters, Sorts, and Fields against the schema's capabilities and
// returns a typed error (wrapping the offending field name) on the first
// violation. A zero Schema validates nothing (back-compat for schema-less
// callers); see Schema.IsZero.
//
// Dotted ("relation.path") field names are skipped here: they reference another
// entity and are validated by the engine against the target schema (TODO), not
// the root schema. Relationship ("has") filters are likewise skipped.
func (s Schema) ValidateQuery(q Query) error {
	if s.IsZero() {
		return nil
	}

	for _, f := range q.Filters {
		if err := s.validateFilter(f); err != nil {
			return err
		}
	}
	for _, sort := range q.Sorts {
		if sort == nil {
			continue
		}
		if err := s.validateField(sort.Column.String(), Sortable, ErrFieldNotSortable); err != nil {
			return err
		}
	}
	for _, field := range q.Fields {
		if err := s.validateField(field.String(), Queryable, ErrFieldNotQueryable); err != nil {
			return err
		}
	}
	return nil
}

// validateFilter recurses through AND/OR groups and validates each leaf field as
// filterable. Relationship ("has") filters are skipped (resolved by the driver
// against the target entity).
func (s Schema) validateFilter(f *FilterSpec) error {
	if f == nil || f.Relation != "" {
		return nil
	}
	if len(f.And) > 0 {
		for _, child := range f.And {
			if err := s.validateFilter(child); err != nil {
				return err
			}
		}
		return nil
	}
	if len(f.Or) > 0 {
		for _, child := range f.Or {
			if err := s.validateFilter(child); err != nil {
				return err
			}
		}
		return nil
	}
	if f.Field == nil {
		return nil
	}
	return s.validateField(f.Field.String(), Filterable, ErrFieldNotFilterable)
}

// validateField checks a single referenced field name: unknown fields yield
// ErrUnknownField; a known field lacking the required capability yields capErr.
// Empty and dotted (relation-path) names are skipped — see ValidateQuery.
func (s Schema) validateField(name string, required Capability, capErr error) error {
	if name == "" || strings.Contains(name, ".") {
		return nil
	}
	a, ok := s.Lookup(name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownField, name)
	}
	if !a.Has(required) {
		return fmt.Errorf("%w: %q", capErr, name)
	}
	return nil
}
