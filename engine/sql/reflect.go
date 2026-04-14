package enginesql

import (
	"fmt"
	"reflect"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// RelationKind describes the type of relationship between two entities.
// Alias of r3tag.RelationKind.
type RelationKind = r3tag.RelationKind

const (
	// RelHasMany represents a one-to-many relationship (e.g. City has many Translations).
	RelHasMany = r3tag.RelHasMany
	// RelBelongsTo represents a many-to-one relationship (e.g. Location belongs to City).
	RelBelongsTo = r3tag.RelBelongsTo
	// RelManyToMany represents a many-to-many relationship via a join table.
	RelManyToMany = r3tag.RelManyToMany
)

// RelationMeta holds metadata about a struct field that represents a relation.
type RelationMeta struct {
	FieldName  string       // Go struct field name (matched against PreloadSpec.Name)
	FieldIndex int          // struct field index for reflection-based assignment
	Kind       RelationKind // has-many, belongs-to, or many-to-many
	FKColumn   string       // foreign key column name (on the "many" side, or left FK for M2M)
	RefColumn  string       // right FK column in join table (M2M only)
	JoinTable  string       // join table name (M2M only)
	Owned      bool         // if true, children are lifecycle-bound to parent (has-many only)
	TargetMeta StructMeta   // metadata for the related entity type
	TargetType reflect.Type // reflect.Type of the target entity (element type, not slice/ptr)
}

// StructMeta holds reflection-based metadata about a struct type T.
// It is used by BaseCRUD and BaseRaw to build SQL queries and scan results.
type StructMeta struct {
	TableName string   // e.g. "cities"
	Columns   []string // column names in order, e.g. ["id", "name", "country_name", ...]
	Fields    []int    // corresponding struct field indices for each column
	PKColumn  string   // primary key column name (defaults to "id")
	PKField   int      // index into Columns/Fields for the PK entry

	// SoftDeleteColumn is the column name used for soft-delete (e.g. "deleted_at").
	// Empty string means soft-delete is not enabled.
	// Detected via the `r3:"soft_delete"` struct tag.
	SoftDeleteColumn string

	// Relations holds metadata about related entities detected via `r3` struct tags.
	// Example tags: `r3:"rel:has-many,fk:city_id"`, `r3:"rel:belongs-to,fk:city_id"`.
	Relations []RelationMeta
}

// GetStructMeta derives table name and column info from a generic type T.
//
// Tag priority: `r3` tag is checked first, `db` tag is used as fallback.
// Fields with tag value "-" (in either tag) are ignored.
// Pointer-to-basic types are kept (nullable columns); slices, maps, and
// struct fields (except time.Time) are treated as relation fields.
func GetStructMeta[T any]() StructMeta {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, true)
}

// getStructMetaForType derives StructMeta from a reflect.Type.
// Used internally for relation target types where we don't need to parse relations
// recursively (avoids deep nesting, only column metadata is needed).
func getStructMetaForType(typ reflect.Type) StructMeta {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, false)
}

// buildStructMeta is the shared implementation for struct metadata extraction.
// When parseRelations is true, relation fields (slices, pointer-to-struct) are
// inspected for `r3` relation tags. When false, they are simply skipped.
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

		// Determine if this field looks like a relation (by its Go type).
		if r3utils.IsRelationType(field.Type) {
			if parseRelations {
				if rel, ok := buildRelationMeta(field, i); ok {
					meta.Relations = append(meta.Relations, rel)
				}
			}
			continue
		}

		// Parse column tag info (r3 first, db fallback).
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

// buildRelationMeta parses the `r3` struct tag on a relation field and builds
// the full RelationMeta including the target type's StructMeta.
func buildRelationMeta(field reflect.StructField, fieldIndex int) (RelationMeta, bool) {
	tag, ok := r3tag.ParseRelationTag(field)
	if !ok {
		return RelationMeta{}, false
	}

	targetType := r3utils.ResolveElementType(field.Type)

	// Build StructMeta for the target type (without parsing its relations
	// to avoid deep/infinite recursion).
	targetMeta := getStructMetaForType(targetType)
	if tag.TableName != "" {
		targetMeta.TableName = tag.TableName
	}

	return RelationMeta{
		FieldName:  field.Name,
		FieldIndex: fieldIndex,
		Kind:       tag.Kind,
		FKColumn:   tag.FKColumn,
		RefColumn:  tag.RefColumn,
		JoinTable:  tag.JoinTable,
		Owned:      tag.Owned,
		TargetMeta: targetMeta,
		TargetType: targetType,
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

// FieldIndicesForColumns returns the subset indices into Columns/Fields that match
// the given column names. It also returns the matching column names (preserving order).
// If selectedCols is empty, returns all indices.
func (m *StructMeta) FieldIndicesForColumns(selectedCols []string) ([]string, []int) {
	if len(selectedCols) == 0 {
		return m.Columns, m.Fields
	}
	selected := make(map[string]bool, len(selectedCols))
	for _, c := range selectedCols {
		selected[c] = true
	}
	// Always include PK column for identity
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

// ScanDestForColumns returns scan destinations for only the specified columns.
// If selectedCols is empty, behaves like ScanDest (all columns).
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

// FieldValuesForColumns extracts field values from an entity for the given column names.
// The returned values are in the same order as the provided columns.
// Columns that don't exist in the struct are silently skipped (validate beforehand).
func (m *StructMeta) FieldValuesForColumns(entity any, columns []string) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	// Build column→field-index lookup
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

// ValidatePatchColumns checks that the given columns are valid for a Patch operation.
// It returns an error if:
//   - columns is empty
//   - any column does not exist in the struct
//   - any column is the primary key
//   - any column is the soft-delete column
//
// On success, it returns the validated column list (same as input).
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
