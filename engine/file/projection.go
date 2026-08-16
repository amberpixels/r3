package enginefile

import (
	"reflect"

	"github.com/amberpixels/r3"
)

// ValidateProjection checks a query's projection against this type's stored
// field names. The file engine derives those from `json`/`yaml` tags as often as
// from `r3`/`db` ones, so the meta - not [r3.SchemaOf] - is the authority on
// what a name has to match.
func (m *StructMeta) ValidateProjection(q r3.Query) error {
	return q.ValidateProjectionAgainst(m.Fields)
}

// keptFieldIndices resolves a query's projection into the struct field indices to
// keep. ok is false when neither projection form is set, so callers skip the
// reflection walk entirely.
//
// The primary key survives every projection, matching the SQL and Mongo engines:
// an entity handed back without its identity cannot be patched, deleted, or
// linked.
func (m *StructMeta) keptFieldIndices(q r3.Query) (map[int]bool, bool) {
	switch {
	case len(q.Fields) > 0:
		named := make(map[string]bool, len(q.Fields))
		for _, name := range r3.FieldsToStrings(q.Fields) {
			named[name] = true
		}
		kept := make(map[int]bool, len(named)+1)
		for i, f := range m.Fields {
			if named[f] || f == m.PKField {
				kept[m.FieldIndices[i]] = true
			}
		}
		return kept, true

	case len(q.ExcludeFields) > 0:
		excluded := make(map[string]bool, len(q.ExcludeFields))
		for _, name := range r3.FieldsToStrings(q.ExcludeFields) {
			excluded[name] = true
		}
		delete(excluded, m.PKField)
		kept := make(map[int]bool, len(m.Fields))
		for i, f := range m.Fields {
			if !excluded[f] {
				kept[m.FieldIndices[i]] = true
			}
		}
		return kept, true

	default:
		return nil, false
	}
}

// projectEntities blanks the fields a query's projection did not ask for, in
// place.
//
// A file-backed record is decoded whole - there is no column list to narrow at
// read time - so projection here subtracts after the fact rather than reading
// less. It still decides what the caller receives, and that has to match the
// other engines: the same query must not hand back a field on one backend that
// it dropped on another.
func projectEntities[T any](entities []T, meta *StructMeta, q r3.Query) {
	kept, ok := meta.keptFieldIndices(q)
	if !ok {
		return
	}
	for i := range entities {
		v := reflect.ValueOf(&entities[i]).Elem()
		for _, idx := range meta.FieldIndices {
			if kept[idx] {
				continue
			}
			if f := v.Field(idx); f.CanSet() {
				f.SetZero()
			}
		}
	}
}
