package i18n

import (
	"fmt"
	"reflect"
)

// Translated describes one preloaded relation whose entities carry translatable
// text. Build it with [TranslatedRelation]; attach it with [WithPreloads].
//
// It is type-erased on purpose. A CRUD[T, ID] has no type parameter for its
// children, so the child's type is resolved inside [TranslatedRelation] - where
// it is still a real type - and reduced to a field map, an id extractor, and the
// reflect.Type used to check the parent's field at startup.
type Translated struct {
	// field is the Go field name on the parent, named the way r3.Preloads names it.
	field string
	// entityType names the child in Translation rows.
	entityType string
	// fields maps the child's translatable storage names to its field indexes.
	fields map[string]int
	// idOf reads a child's Translation.EntityID out of its struct value.
	idOf func(reflect.Value) string
	// childType is the child struct type, checked against the parent's field.
	childType reflect.Type
	// fieldIndex is the parent's field index, filled in by WithTranslations.
	fieldIndex int
	// err is a build-time failure, reported by WithTranslations so that every
	// wiring error surfaces from the same place.
	err error
}

// TranslatedRelation declares that the preloaded relation at the parent's field
// carries translatable text, naming the child's translatable fields by storage
// name (db-tag name or snake_cased Go name) and how to read the child's id.
//
// field is the Go field name, as in r3.Preloads("City"). The relation's field on
// the parent may be C, *C, []C or []*C.
//
// This is a read overlay: it never writes to the child's translations, and a
// parent's Update/Patch does not mark them stale. That belongs to whatever repo
// owns the child.
func TranslatedRelation[C any, CID comparable](
	field string, idFunc func(C) CID, fields ...string,
) Translated {
	rel := Translated{
		field:      field,
		entityType: deriveEntityType[C](),
		childType:  reflect.TypeFor[C](),
		fieldIndex: -1,
	}

	switch {
	case field == "":
		rel.err = fmt.Errorf("%w: a translated relation needs a field name", ErrNotTranslatable)
		return rel
	case idFunc == nil:
		rel.err = fmt.Errorf("%w: relation %q needs an id function", ErrNotTranslatable, field)
		return rel
	case len(fields) == 0:
		rel.err = fmt.Errorf("%w: relation %q names no translatable fields", ErrNotTranslatable, field)
		return rel
	}

	resolved, err := resolveFields[C](fields)
	if err != nil {
		rel.err = fmt.Errorf("relation %q: %w", field, err)
		return rel
	}
	rel.fields = resolved
	rel.idOf = func(v reflect.Value) string {
		child, ok := reflect.TypeAssert[C](v)
		if !ok {
			return ""
		}
		return fmt.Sprint(idFunc(child))
	}
	return rel
}

// locatedChild is one child entity found on the page, paired with the id its
// translations are keyed by.
type locatedChild struct {
	value reflect.Value
	id    string
}

// resolvePreloads checks each declared relation against T and resolves the
// parent's field index. Called once at wrap time, so a mis-declared relation
// fails at startup rather than serving one language per nesting level.
//
// T is always a struct here: resolveFields rejects anything else, and
// WithTranslations runs it first.
func resolvePreloads[T any](rels []Translated) ([]Translated, error) {
	if len(rels) == 0 {
		return nil, nil
	}

	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %s is not a struct", ErrNotTranslatable, t)
	}

	out := make([]Translated, 0, len(rels))
	for _, rel := range rels {
		if rel.err != nil {
			return nil, rel.err
		}
		field, ok := t.FieldByName(rel.field)
		if !ok || !field.IsExported() {
			return nil, fmt.Errorf(
				"%w: no exported field %q on %s", ErrNotTranslatable, rel.field, t.Name(),
			)
		}
		if len(field.Index) != 1 {
			return nil, fmt.Errorf(
				"%w: field %q on %s is promoted from an embedded type; declare the relation on the type that holds it",
				ErrNotTranslatable, rel.field, t.Name(),
			)
		}
		if !holdsChild(field.Type, rel.childType) {
			return nil, fmt.Errorf(
				"%w: field %q on %s is %s, want %s, *%s, []%s or []*%s",
				ErrNotTranslatable, rel.field, t.Name(), field.Type,
				rel.childType, rel.childType, rel.childType, rel.childType,
			)
		}
		rel.fieldIndex = field.Index[0]
		out = append(out, rel)
	}
	return out, nil
}

// holdsChild reports whether a parent field can hold child entities: C, *C,
// []C or []*C.
func holdsChild(fieldType, child reflect.Type) bool {
	if fieldType.Kind() == reflect.Slice {
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() == reflect.Pointer {
		fieldType = fieldType.Elem()
	}
	return fieldType == child
}

// relationChildren returns the addressable child structs a relation field holds:
// one for C or *C, each element for []C or []*C. Nil pointers and empty slices
// yield none.
func relationChildren(field reflect.Value) []reflect.Value {
	if field.Kind() == reflect.Slice {
		out := make([]reflect.Value, 0, field.Len())
		for i := range field.Len() {
			if child, ok := derefChild(field.Index(i)); ok {
				out = append(out, child)
			}
		}
		return out
	}
	if child, ok := derefChild(field); ok {
		return []reflect.Value{child}
	}
	return nil
}

// derefChild unwraps a pointer child, reporting false for a nil one.
func derefChild(v reflect.Value) (reflect.Value, bool) {
	if v.Kind() != reflect.Pointer {
		return v, true
	}
	if v.IsNil() {
		return reflect.Value{}, false
	}
	return v.Elem(), true
}
