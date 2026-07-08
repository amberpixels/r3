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

// ValidateAggregateQuery is the aggregate counterpart of ValidateQuery, called
// by engines before building an Aggregate. The structural checks (aggregates
// declared, aliases valid and unique, Having references declared names) run
// even for a zero Schema; capability checks (group and aggregated fields must
// be Filterable, SUM/AVG need a numeric attribute) apply only when a schema is
// present, keeping the permissive-defaults philosophy.
//
// It does not check Sorts: sorts that reference neither a group field nor an
// alias are dropped by Query.AggregateSorts, not rejected — they are usually
// inherited repo defaults, invisible to the caller.
func (s Schema) ValidateAggregateQuery(q Query) error {
	if err := validateAggregateShape(q); err != nil {
		return err
	}
	if s.IsZero() {
		return nil
	}

	for _, f := range q.Filters {
		if err := s.validateFilter(f); err != nil {
			return err
		}
	}
	for _, g := range q.GroupBy {
		if err := s.validateField(g.String(), Filterable, ErrFieldNotFilterable); err != nil {
			return err
		}
	}
	for _, a := range q.Aggregates {
		if a == nil || a.Field == nil {
			continue
		}
		name := a.Field.String()
		if err := s.validateField(name, Filterable, ErrFieldNotFilterable); err != nil {
			return err
		}
		if a.Func == AggregateSum || a.Func == AggregateAvg {
			// Only reject when the schema KNOWS the type is non-numeric; an
			// untyped attribute (hand-built schema) stays permissive.
			if attr, ok := s.Lookup(name); ok && attr.Type != "" && attr.Type != TypeInt && attr.Type != TypeFloat {
				return fmt.Errorf("%w: %s over non-numeric field %q", ErrInvalidAggregate, a.Func, name)
			}
		}
	}
	return nil
}

// validateAggregateShape enforces the schema-independent structure of an
// aggregate query: at least one aggregate, valid unique aliases that don't
// shadow group fields, fields where the function requires one, and Having
// limited to declared aliases and group fields.
func validateAggregateShape(q Query) error {
	if len(q.Aggregates) == 0 {
		return fmt.Errorf("%w: at least one aggregate is required", ErrInvalidAggregate)
	}

	groups := make(map[string]struct{}, len(q.GroupBy))
	for _, g := range q.GroupBy {
		if err := ValidateIdentifier(g.String()); err != nil {
			return fmt.Errorf("%w: invalid group field %q", ErrInvalidAggregate, g.String())
		}
		groups[g.String()] = struct{}{}
	}

	names := make(map[string]struct{}, len(q.Aggregates)+len(groups))
	for g := range groups {
		names[g] = struct{}{}
	}
	for _, a := range q.Aggregates {
		if a == nil {
			return fmt.Errorf("%w: nil aggregate spec", ErrInvalidAggregate)
		}
		if !isValidIdentifierSegment(a.Alias) {
			return fmt.Errorf("%w: invalid alias %q", ErrInvalidAggregate, a.Alias)
		}
		if _, dup := names[a.Alias]; dup {
			return fmt.Errorf("%w: duplicate alias %q", ErrInvalidAggregate, a.Alias)
		}
		names[a.Alias] = struct{}{}
		switch a.Func {
		case AggregateCount:
			// COUNT(*) — field optional.
		case AggregateCountDistinct, AggregateSum, AggregateAvg, AggregateMin, AggregateMax:
			if a.Field.String() == "" {
				return fmt.Errorf("%w: %s requires a field (alias %q)", ErrInvalidAggregate, a.Func, a.Alias)
			}
		default:
			return fmt.Errorf("%w: unknown aggregate function (alias %q)", ErrInvalidAggregate, a.Alias)
		}
		if a.Field != nil && a.Field.String() != "" {
			if err := ValidateIdentifier(a.Field.String()); err != nil {
				return fmt.Errorf("%w: invalid aggregate field %q", ErrInvalidAggregate, a.Field.String())
			}
		}
	}

	for _, h := range q.Having {
		if err := validateHavingFilter(h, names); err != nil {
			return err
		}
	}
	return nil
}

// validateHavingFilter recurses AND/OR groups and requires every leaf to
// reference a declared aggregate alias or group field. Relationship filters
// have no meaning over grouped rows.
func validateHavingFilter(f *FilterSpec, names map[string]struct{}) error {
	if f == nil {
		return nil
	}
	if f.Relation != "" {
		return fmt.Errorf("%w: relationship filter in Having", ErrInvalidAggregate)
	}
	for _, child := range f.And {
		if err := validateHavingFilter(child, names); err != nil {
			return err
		}
	}
	for _, child := range f.Or {
		if err := validateHavingFilter(child, names); err != nil {
			return err
		}
	}
	if len(f.And) > 0 || len(f.Or) > 0 || f.Field == nil {
		return nil
	}
	if _, ok := names[f.Field.String()]; !ok {
		return fmt.Errorf("%w: Having references undeclared name %q", ErrInvalidAggregate, f.Field.String())
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
