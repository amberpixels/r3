package history

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// relationField is a relation history records: a many-to-many or an owned
// has-many, the two kinds a driver writes through the parent. A belongs-to is
// already recorded through its FK column; a non-owned has-many is never written
// through the parent.
type relationField struct {
	index  int    // struct field index on the owner
	goName string // Go field name, which is also the preload name
	name   string // recorded field name (snake_case of goName)
	m2m    bool   // many-to-many; otherwise an owned has-many

	// ordered is true when the backend persists the slice order (an `order:`
	// join-table column), so a pure reorder is a real change.
	ordered bool

	elem    reflect.Type // the related struct type
	elemPtr bool         // the slice holds *elem rather than elem

	// m2m: the related type's PK field index, -1 when it has none.
	targetPK int
	// owned has-many: the child columns a member is identified by.
	childCols []childColumn
}

// childColumn is one recorded column of an owned has-many child.
type childColumn struct {
	index  int
	column string
}

// Owned children are re-inserted on every sync, so the columns the insert
// rewrites (PK, FK to the parent, managed timestamps) say nothing about what the
// caller changed and are left out of a child's identity.
var churnedChildColumns = map[string]bool{"created_at": true, "updated_at": true}

var relationCache sync.Map // reflect.Type -> []relationField

// relationFieldsOf returns the recorded relations of struct type t, cached.
func relationFieldsOf(t reflect.Type) []relationField {
	if cached, ok := relationCache.Load(t); ok {
		rels, _ := cached.([]relationField)
		return rels
	}

	var rels []relationField
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() || field.Type.Kind() != reflect.Slice {
			continue
		}
		tag, ok := r3tag.ParseRelationTag(field)
		if !ok {
			continue
		}
		m2m := tag.Kind == r3tag.RelManyToMany
		if !m2m && (tag.Kind != r3tag.RelHasMany || !tag.Owned) {
			continue
		}
		elem := r3utils.ResolveElementType(field.Type)
		if elem.Kind() != reflect.Struct {
			continue
		}

		rel := relationField{
			index:    i,
			goName:   field.Name,
			name:     r3utils.ToSnakeCase(field.Name),
			m2m:      m2m,
			ordered:  m2m && tag.OrderColumn != "",
			elem:     elem,
			elemPtr:  field.Type.Elem().Kind() == reflect.Pointer,
			targetPK: -1,
		}
		pk := -1
		var cols []childColumn
		for j := range elem.NumField() {
			f := elem.Field(j)
			if !f.IsExported() || r3utils.IsRelationType(f.Type) {
				continue
			}
			ct := r3tag.ParseColumnTag(f)
			if ct.Skip {
				continue
			}
			if ct.IsPK || ct.Column == "id" {
				pk = j
			}
			cols = append(cols, childColumn{index: j, column: ct.Column})
		}
		if m2m {
			if pk < 0 {
				continue // no identity to record members by
			}
			rel.targetPK = pk
		} else {
			for _, c := range cols {
				if c.index == pk || c.column == tag.FKColumn || churnedChildColumns[c.column] {
					continue
				}
				rel.childCols = append(rel.childCols, c)
			}
		}
		rels = append(rels, rel)
	}

	relationCache.Store(t, rels)
	return rels
}

// structValue dereferences v down to its struct, or reports false.
func structValue(v reflect.Value) (reflect.Value, bool) {
	for v.IsValid() && v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, false
		}
		v = v.Elem()
	}
	return v, v.IsValid() && v.Kind() == reflect.Struct
}

// members returns a relation slice as recorded member values: target PKs for a
// many-to-many, column-keyed maps for an owned has-many. A nil slice ("not
// loaded") returns nil; a non-nil empty one returns an empty, non-nil list.
func (rel relationField) members(slice reflect.Value) []any {
	if slice.IsNil() {
		return nil
	}
	out := make([]any, 0, slice.Len())
	for i := range slice.Len() {
		elem, ok := structValue(slice.Index(i))
		if !ok {
			continue
		}
		if rel.m2m {
			out = append(out, memberValue(elem.Field(rel.targetPK)))
			continue
		}
		child := make(map[string]any, len(rel.childCols))
		for _, c := range rel.childCols {
			child[c.column] = memberValue(elem.Field(c.index))
		}
		out = append(out, child)
	}
	return out
}

// memberValue is a member's plain value: pointers dereferenced, and times in UTC
// with no monotonic reading, so a value read back from the DB compares equal to
// the one the caller wrote.
func memberValue(v reflect.Value) any {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if t, ok := reflect.TypeAssert[time.Time](v); ok {
		return t.UTC()
	}
	return v.Interface()
}

// sameMembers reports whether two member lists record the same relation state:
// equal in order for an ordered relation, equal as multisets otherwise.
func (rel relationField) sameMembers(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	ka, kb := memberKeys(a), memberKeys(b)
	if rel.ordered {
		for i := range ka {
			if ka[i] != kb[i] {
				return false
			}
		}
		return true
	}
	counts := make(map[string]int, len(ka))
	for _, k := range ka {
		counts[k]++
	}
	for _, k := range kb {
		counts[k]--
		if counts[k] < 0 {
			return false
		}
	}
	return true
}

// memberKeys renders members as JSON, the form they are stored in, so equality
// matches what a reader of the record would see.
func memberKeys(members []any) []string {
	keys := make([]string, len(members))
	for i, m := range members {
		b, err := json.Marshal(m)
		if err != nil {
			keys[i] = fmt.Sprintf("%#v", m)
			continue
		}
		keys[i] = string(b)
	}
	return keys
}

// diffRelations appends a change for each recorded relation whose membership
// differs between oldVal and newVal. Nil on either side is "not loaded", so
// nothing is recorded: an unknown side must never read as a clear or a first
// attachment. An empty new list records an explicit clear.
func diffRelations(oldVal, newVal reflect.Value, changes *[]FieldChange) {
	for _, rel := range relationFieldsOf(oldVal.Type()) {
		oldSlice, newSlice := oldVal.Field(rel.index), newVal.Field(rel.index)
		if oldSlice.IsNil() || newSlice.IsNil() {
			continue
		}
		oldMembers, newMembers := rel.members(oldSlice), rel.members(newSlice)
		if rel.sameMembers(oldMembers, newMembers) {
			continue
		}
		*changes = append(*changes, FieldChange{Field: rel.name, OldValue: oldMembers, NewValue: newMembers})
	}
}

// createRelations records nil -> members for every loaded relation, so the create
// carries the relation baseline reconstruction replays from.
func createRelations(val reflect.Value, changes *[]FieldChange) {
	for _, rel := range relationFieldsOf(val.Type()) {
		if slice := val.Field(rel.index); !slice.IsNil() {
			*changes = append(*changes, FieldChange{Field: rel.name, OldValue: nil, NewValue: rel.members(slice)})
		}
	}
}

// deleteRelations records members -> nil for every loaded relation.
func deleteRelations(val reflect.Value, changes *[]FieldChange) {
	for _, rel := range relationFieldsOf(val.Type()) {
		if slice := val.Field(rel.index); !slice.IsNil() {
			*changes = append(*changes, FieldChange{Field: rel.name, OldValue: rel.members(slice), NewValue: nil})
		}
	}
}

// relationPreloads returns the Get query that loads the old state of every
// relation entity carries non-nil - exactly the relations a write will sync -
// or no query when it carries none, so a save that writes no relation reads
// nothing extra.
func relationPreloads[T any](entity T) []r3.Query {
	val, ok := structValue(reflect.ValueOf(entity))
	if !ok {
		return nil
	}
	var preloads r3.Preloads
	for _, rel := range relationFieldsOf(val.Type()) {
		if !val.Field(rel.index).IsNil() {
			preloads = append(preloads, r3.NewPreloadSpec(rel.goName))
		}
	}
	if len(preloads) == 0 {
		return nil
	}
	return []r3.Query{{Preloads: preloads}}
}

// splitRelationChanges separates relation changes, which replay by reflection,
// from the field changes that replay through the JSON round-trip.
func splitRelationChanges(t reflect.Type, changes []FieldChange) ([]FieldChange, map[string]FieldChange) {
	rels := relationFieldsOf(t)
	if len(rels) == 0 {
		return changes, nil
	}
	names := make(map[string]bool, len(rels))
	for _, rel := range rels {
		names[rel.name] = true
	}
	var scalar []FieldChange
	var relational map[string]FieldChange
	for _, c := range changes {
		if !names[c.Field] {
			scalar = append(scalar, c)
			continue
		}
		if relational == nil {
			relational = make(map[string]FieldChange)
		}
		relational[c.Field] = c
	}
	return scalar, relational
}

// applyRelationChanges sets each changed relation of the struct val to its
// recorded new members: target stubs carrying only the PK for a many-to-many
// (all a join-table sync writes), children rebuilt from their recorded columns
// for an owned has-many. A nil new value sets the relation to nil.
func applyRelationChanges(val reflect.Value, changes map[string]FieldChange) error {
	for _, rel := range relationFieldsOf(val.Type()) {
		c, ok := changes[rel.name]
		if !ok {
			continue
		}
		field := val.Field(rel.index)
		members, ok := asList(c.NewValue)
		if !ok {
			field.SetZero()
			continue
		}
		slice := reflect.MakeSlice(field.Type(), 0, len(members))
		for _, m := range members {
			elem := reflect.New(rel.elem)
			if err := rel.fillMember(elem.Elem(), m); err != nil {
				return fmt.Errorf("relation %s: %w", rel.name, err)
			}
			if rel.elemPtr {
				slice = reflect.Append(slice, elem)
			} else {
				slice = reflect.Append(slice, elem.Elem())
			}
		}
		field.Set(slice)
	}
	return nil
}

// asList reads a recorded member list, which after a store round-trip is a
// []any; nil (a deleted or unloaded relation) reports false.
func asList(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	out := make([]any, rv.Len())
	for i := range rv.Len() {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}

// fillMember writes one recorded member into a zero related struct.
func (rel relationField) fillMember(elem reflect.Value, member any) error {
	if rel.m2m {
		return setFromJSON(elem.Field(rel.targetPK), member)
	}
	cols, ok := member.(map[string]any)
	if !ok {
		return fmt.Errorf("member %v is not a column map", member)
	}
	for _, c := range rel.childCols {
		if v, ok := cols[c.column]; ok {
			if err := setFromJSON(elem.Field(c.index), v); err != nil {
				return fmt.Errorf("column %s: %w", c.column, err)
			}
		}
	}
	return nil
}

// setFromJSON assigns v to field through JSON, which converts what a store
// round-trip leaves (float64 numbers, RFC 3339 strings) back to the field's type.
func setFromJSON(field reflect.Value, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ptr := reflect.New(field.Type())
	if err := json.Unmarshal(b, ptr.Interface()); err != nil {
		return err
	}
	field.Set(ptr.Elem())
	return nil
}
