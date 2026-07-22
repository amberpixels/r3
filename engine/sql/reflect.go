package enginesql

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// structMetaCache memoizes StructMeta per type. Derivation reflects over the
// whole struct (and its relation targets), so without caching every CRUD op
// would re-reflect; drivers used to call GetStructMeta on each call.
var structMetaCache sync.Map // reflect.Type -> StructMeta

// RelationKind is an alias of r3tag.RelationKind.
type RelationKind = r3tag.RelationKind

const (
	// RelHasMany is a one-to-many relationship (City has many Translations).
	RelHasMany = r3tag.RelHasMany
	// RelBelongsTo is a many-to-one relationship (Location belongs to City).
	RelBelongsTo = r3tag.RelBelongsTo
	// RelManyToMany is a many-to-many relationship via a join table.
	RelManyToMany = r3tag.RelManyToMany
)

// RelationMeta describes a struct field that represents a relation.
type RelationMeta struct {
	FieldName  string       // Go field name (matched against PreloadSpec.Name)
	FieldIndex int          // struct field index for reflection-based assignment
	Kind       RelationKind // has-many, belongs-to, or many-to-many
	FKColumn   string       // FK column (on the "many" side, or left FK for M2M)
	RefColumn  string       // right FK column in join table (M2M only)
	JoinTable  string       // join table name (M2M only)
	// OrderColumn is an integer join-table column persisting the slice order
	// (M2M only): sync writes each element's index, preload orders by it.
	// Empty means order is not persisted.
	OrderColumn string
	Owned       bool         // children are lifecycle-bound to parent (has-many only)
	TargetMeta  StructMeta   // metadata for the related entity type
	TargetType  reflect.Type // target element type (not slice/ptr)
}

// StructMeta is the reflected metadata for a struct type T, used by BaseCRUD and
// BaseRaw to build queries and scan results.
type StructMeta struct {
	TableName string   // e.g. "cities"
	Columns   []string // column names in order
	Fields    []int    // struct field index per column
	PKColumn  string   // primary key column (defaults to "id")
	PKField   int      // index into Columns/Fields for the PK entry

	// SoftDeleteColumn is the soft-delete column (e.g. "deleted_at"), detected via
	// `r3:"soft_delete"`. Empty means soft-delete is off.
	SoftDeleteColumn string

	// Relations holds relations detected via `r3` tags, e.g.
	// `r3:"rel:has-many,fk:city_id"`.
	Relations []RelationMeta
}

// GetStructMeta derives table name and columns from T.
//
// Tag priority: `r3` first, `db` as fallback; a "-" value in either is ignored.
// Pointer-to-basic types are nullable columns; slices, maps, and struct fields
// (except time.Time) are relation fields.
func GetStructMeta[T any]() StructMeta {
	typ := reflect.TypeFor[T]()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if cached, ok := structMetaCache.Load(typ); ok {
		if meta, ok := cached.(StructMeta); ok {
			return meta
		}
	}
	meta := buildStructMeta(typ, true)
	structMetaCache.Store(typ, meta)
	return meta
}

// getStructMetaForType derives StructMeta from a reflect.Type without parsing
// relations - used for relation targets, where only column metadata is needed
// (and recursion must stop).
func getStructMetaForType(typ reflect.Type) StructMeta {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, false)
}

// buildStructMeta extracts struct metadata. With parseRelations, relation fields
// (slices, pointer-to-struct) are inspected for `r3` relation tags; otherwise
// skipped.
func buildStructMeta(typ reflect.Type, parseRelations bool) StructMeta {
	meta := StructMeta{
		TableName: r3utils.ToSnakeCasePlural(typ.Name()),
		PKColumn:  "id",
		PKField:   -1,
	}

	for i := range typ.NumField() {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		// A relation field (by Go type): parse or skip.
		if r3utils.IsRelationType(field.Type) {
			if parseRelations {
				if rel, ok := buildRelationMeta(field, i); ok {
					meta.Relations = append(meta.Relations, rel)
				}
			}
			continue
		}

		tag := r3tag.ParseColumnTag(field)
		if tag.Skip {
			continue
		}

		meta.Columns = append(meta.Columns, tag.Column)
		meta.Fields = append(meta.Fields, i)

		if tag.IsPK || tag.Column == "id" {
			meta.PKColumn = tag.Column
			meta.PKField = len(meta.Fields) - 1
		}

		if tag.SoftDelete {
			meta.SoftDeleteColumn = tag.Column
		}
	}

	return meta
}

// buildRelationMeta parses a relation field's `r3` tag into a RelationMeta,
// including the target type's StructMeta.
func buildRelationMeta(field reflect.StructField, fieldIndex int) (RelationMeta, bool) {
	tag, ok := r3tag.ParseRelationTag(field)
	if !ok {
		return RelationMeta{}, false
	}

	targetType := r3utils.ResolveElementType(field.Type)

	// No nested relations, to avoid deep/infinite recursion.
	targetMeta := getStructMetaForType(targetType)
	if tag.TableName != "" {
		targetMeta.TableName = tag.TableName
	}

	return RelationMeta{
		FieldName:   field.Name,
		FieldIndex:  fieldIndex,
		Kind:        tag.Kind,
		FKColumn:    tag.FKColumn,
		RefColumn:   tag.RefColumn,
		JoinTable:   tag.JoinTable,
		OrderColumn: tag.OrderColumn,
		Owned:       tag.Owned,
		TargetMeta:  targetMeta,
		TargetType:  targetType,
	}, true
}

// --------------------------------------------------------------------------
// StructMeta methods
// --------------------------------------------------------------------------

// NonPKColumns returns all column names except the primary key.
func (m *StructMeta) NonPKColumns() []string {
	var cols []string
	for _, c := range m.Columns {
		if c != m.PKColumn {
			cols = append(cols, c)
		}
	}
	return cols
}

// FieldValues extracts the column values from a struct in the same order as Columns.
func (m *StructMeta) FieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	vals := make([]any, len(m.Fields))
	for i, idx := range m.Fields {
		vals[i] = v.Field(idx).Interface()
	}
	return vals
}

// NonPKFieldValues extracts column values excluding the PK, for INSERT/UPDATE.
func (m *StructMeta) NonPKFieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	var vals []any
	for i, idx := range m.Fields {
		if m.Columns[i] != m.PKColumn {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// ScanDest returns a slice of pointers to the struct fields, suitable for sql.Row.Scan().
// entityPtr must be a pointer to the entity.
func (m *StructMeta) ScanDest(entityPtr any) []any {
	v := reflect.ValueOf(entityPtr).Elem()
	dests := make([]any, len(m.Fields))
	for i, idx := range m.Fields {
		dests[i] = v.Field(idx).Addr().Interface()
	}
	return dests
}

// PKValue extracts the primary key value from an entity.
func (m *StructMeta) PKValue(entity any) any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if m.PKField >= 0 && m.PKField < len(m.Fields) {
		return v.Field(m.Fields[m.PKField]).Interface()
	}
	return nil
}

// SetPKValue sets the primary key value on an entity (via pointer).
func (m *StructMeta) SetPKValue(entityPtr any, val any) {
	v := reflect.ValueOf(entityPtr).Elem()
	if m.PKField >= 0 && m.PKField < len(m.Fields) {
		pkField := v.Field(m.Fields[m.PKField])
		pkField.Set(reflect.ValueOf(val).Convert(pkField.Type()))
	}
}

// FieldIndicesForColumns returns the Columns/Fields subset (columns and their
// field indices, in order) matching selectedCols; all of them when empty.
func (m *StructMeta) FieldIndicesForColumns(selectedCols []string) ([]string, []int) {
	if len(selectedCols) == 0 {
		return m.Columns, m.Fields
	}
	selected := make(map[string]bool, len(selectedCols))
	for _, c := range selectedCols {
		selected[c] = true
	}
	// Always include the PK for identity.
	selected[m.PKColumn] = true

	var columns []string
	var fieldIndices []int
	for i, col := range m.Columns {
		if selected[col] {
			columns = append(columns, col)
			fieldIndices = append(fieldIndices, m.Fields[i])
		}
	}
	return columns, fieldIndices
}

// ScanDestForColumns returns scan destinations for selectedCols only; like
// ScanDest (all columns) when empty.
func (m *StructMeta) ScanDestForColumns(entityPtr any, selectedCols []string) []any {
	if len(selectedCols) == 0 {
		return m.ScanDest(entityPtr)
	}
	_, fieldIndices := m.FieldIndicesForColumns(selectedCols)
	v := reflect.ValueOf(entityPtr).Elem()
	dests := make([]any, len(fieldIndices))
	for i, idx := range fieldIndices {
		dests[i] = v.Field(idx).Addr().Interface()
	}
	return dests
}

// FieldValuesForColumns extracts an entity's values for the given columns, in
// order. Columns absent from the struct are silently skipped (validate first).
func (m *StructMeta) FieldValuesForColumns(entity any, columns []string) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	colToField := make(map[string]int, len(m.Columns))
	for i, col := range m.Columns {
		colToField[col] = m.Fields[i]
	}

	vals := make([]any, 0, len(columns))
	for _, col := range columns {
		if idx, ok := colToField[col]; ok {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// CopyColumnFields overwrites dstPtr's column-backed fields with src's, leaving
// every other field (relations, association pointers, `-`-tagged fields)
// untouched.
//
// Drivers use it to fold a freshly re-read row into the entity they are about to
// return: the columns then carry DB truth (values the write omitted, DB-stamped
// timestamps, defaults, triggers) while associations stay as the caller supplied
// them. A re-read does not load associations, and a nil relation means "not
// loaded" throughout r3 - so copying the row wholesale would silently drop them.
func (m *StructMeta) CopyColumnFields(dstPtr any, src any) {
	dst := reflect.ValueOf(dstPtr)
	if dst.Kind() != reflect.Pointer {
		return
	}
	dst = dst.Elem()

	s := reflect.ValueOf(src)
	if s.Kind() == reflect.Pointer {
		s = s.Elem()
	}

	for _, idx := range m.Fields {
		dst.Field(idx).Set(s.Field(idx))
	}
}

// ValidatePatchColumns is the structural floor for Patch: it rejects an empty
// set, or any unknown, primary-key, or soft-delete column. On success it returns
// columns unchanged.
func (m *StructMeta) ValidatePatchColumns(columns []string) ([]string, error) {
	if len(columns) == 0 {
		return nil, r3.ErrNoPatchFields
	}

	known := make(map[string]bool, len(m.Columns))
	for _, col := range m.Columns {
		known[col] = true
	}

	for _, col := range columns {
		if !known[col] {
			return nil, fmt.Errorf("%w: %q does not exist", r3.ErrInvalidPatchField, col)
		}
		if col == m.PKColumn {
			return nil, fmt.Errorf("%w: %q is the primary key", r3.ErrInvalidPatchField, col)
		}
		if m.SoftDeleteColumn != "" && col == m.SoftDeleteColumn {
			return nil, fmt.Errorf("%w: %q is the soft-delete column", r3.ErrInvalidPatchField, col)
		}
	}

	return columns, nil
}
