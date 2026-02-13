package sqlbase

import (
	"reflect"
	"strings"
)

// RelationKind describes the type of relationship between two entities.
type RelationKind int

const (
	// RelHasMany represents a one-to-many relationship (e.g. City has many Translations).
	// The FK column is on the child table, referencing the parent's PK.
	RelHasMany RelationKind = iota
	// RelBelongsTo represents a many-to-one relationship (e.g. Location belongs to City).
	// The FK column is on the parent table, referencing the child's PK.
	RelBelongsTo
)

// RelationMeta holds metadata about a struct field that represents a relation.
type RelationMeta struct {
	FieldName  string       // Go struct field name (matched against PreloadSpec.Name)
	FieldIndex int          // struct field index for reflection-based assignment
	Kind       RelationKind // has-many or belongs-to
	FKColumn   string       // foreign key column name (on the "many" side)
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
// It looks for `db:"column_name"` struct tags. Fields without a `db` tag
// use the snake_case version of the field name. Fields with `db:"-"` are ignored.
// Pointer-to-basic types are kept (nullable columns); slices, maps, and
// struct fields (except time.Time) are skipped as relations.
func GetStructMeta[T any]() StructMeta {
	var t T
	typ := reflect.TypeOf(t)

	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	meta := StructMeta{
		TableName: ToSnakeCasePlural(typ.Name()),
		PKColumn:  "id",
		PKField:   -1,
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		fKind := field.Type.Kind()

		// Check if this is a relation field (slice or pointer-to-struct, except time.Time).
		// If so, check for an `r3` tag declaring a relation before skipping.
		isRelationField := false
		if fKind == reflect.Slice || fKind == reflect.Map {
			isRelationField = true
		} else if fKind == reflect.Pointer {
			elemKind := field.Type.Elem().Kind()
			if elemKind == reflect.Struct && field.Type.Elem().String() != "time.Time" {
				isRelationField = true
			}
		} else if fKind == reflect.Struct && field.Type.String() != "time.Time" {
			isRelationField = true
		}

		if isRelationField {
			// Check for r3 relation tag on this field
			if rel, ok := parseRelationTag(field, i); ok {
				meta.Relations = append(meta.Relations, rel)
			}
			continue
		}

		// Parse the `db` tag for column fields
		tag := field.Tag.Get("db")
		if tag == "-" {
			continue
		}

		colName := tag
		isPK := false
		if tag != "" {
			parts := strings.Split(tag, ",")
			colName = parts[0]
			for _, part := range parts[1:] {
				if strings.TrimSpace(part) == "pk" {
					isPK = true
				}
			}
		}
		if colName == "" {
			colName = ToSnakeCase(field.Name)
		}

		meta.Columns = append(meta.Columns, colName)
		meta.Fields = append(meta.Fields, i)

		if isPK || colName == "id" {
			meta.PKColumn = colName
			meta.PKField = len(meta.Fields) - 1
		}

		// Parse `r3` tag for soft-delete
		r3Tag := field.Tag.Get("r3")
		if r3Tag != "" {
			for _, part := range strings.Split(r3Tag, ",") {
				part = strings.TrimSpace(part)
				if part == "soft_delete" {
					meta.SoftDeleteColumn = colName
				}
			}
		}
	}

	return meta
}

// parseRelationTag parses the `r3` struct tag on a relation field.
// Returns a RelationMeta and true if the tag declares a valid relation.
// Tag format: `r3:"rel:has-many,fk:city_id"` or `r3:"rel:belongs-to,fk:city_id"`
// Optional table override: `r3:"rel:has-many,fk:city_id,table:my_translations"`
func parseRelationTag(field reflect.StructField, fieldIndex int) (RelationMeta, bool) {
	r3Tag := field.Tag.Get("r3")
	if r3Tag == "" {
		return RelationMeta{}, false
	}

	parts := strings.Split(r3Tag, ",")
	var kind RelationKind
	var hasRel bool
	var fkCol string
	var tableName string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "rel:has-many":
			kind = RelHasMany
			hasRel = true
		case part == "rel:belongs-to":
			kind = RelBelongsTo
			hasRel = true
		case strings.HasPrefix(part, "fk:"):
			fkCol = strings.TrimPrefix(part, "fk:")
		case strings.HasPrefix(part, "table:"):
			tableName = strings.TrimPrefix(part, "table:")
		}
	}

	if !hasRel || fkCol == "" {
		return RelationMeta{}, false
	}

	// Determine the target element type
	var targetType reflect.Type
	switch field.Type.Kind() {
	case reflect.Slice:
		targetType = field.Type.Elem()
		if targetType.Kind() == reflect.Pointer {
			targetType = targetType.Elem()
		}
	case reflect.Pointer:
		targetType = field.Type.Elem()
	default:
		targetType = field.Type
	}

	// Build StructMeta for the target type (recursive, but safe since relations
	// are skipped when they don't have r3 tags — no infinite loops)
	targetMeta := getStructMetaForType(targetType)
	if tableName != "" {
		targetMeta.TableName = tableName
	}

	return RelationMeta{
		FieldName:  field.Name,
		FieldIndex: fieldIndex,
		Kind:       kind,
		FKColumn:   fkCol,
		TargetMeta: targetMeta,
		TargetType: targetType,
	}, true
}

// getStructMetaForType derives StructMeta from a reflect.Type (used for relation targets).
func getStructMetaForType(typ reflect.Type) StructMeta {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	meta := StructMeta{
		TableName: ToSnakeCasePlural(typ.Name()),
		PKColumn:  "id",
		PKField:   -1,
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		fKind := field.Type.Kind()
		if fKind == reflect.Slice || fKind == reflect.Map {
			continue
		}
		if fKind == reflect.Pointer {
			elemKind := field.Type.Elem().Kind()
			if elemKind == reflect.Struct && field.Type.Elem().String() != "time.Time" {
				continue
			}
		}
		if fKind == reflect.Struct && field.Type.String() != "time.Time" {
			continue
		}

		tag := field.Tag.Get("db")
		if tag == "-" {
			continue
		}

		colName := tag
		isPK := false
		if tag != "" {
			parts := strings.Split(tag, ",")
			colName = parts[0]
			for _, part := range parts[1:] {
				if strings.TrimSpace(part) == "pk" {
					isPK = true
				}
			}
		}
		if colName == "" {
			colName = ToSnakeCase(field.Name)
		}

		meta.Columns = append(meta.Columns, colName)
		meta.Fields = append(meta.Fields, i)

		if isPK || colName == "id" {
			meta.PKColumn = colName
			meta.PKField = len(meta.Fields) - 1
		}
	}

	return meta
}

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
func (m *StructMeta) FieldIndicesForColumns(selectedCols []string) (columns []string, fieldIndices []int) {
	if len(selectedCols) == 0 {
		return m.Columns, m.Fields
	}
	selected := make(map[string]bool, len(selectedCols))
	for _, c := range selectedCols {
		selected[c] = true
	}
	// Always include PK column for identity
	selected[m.PKColumn] = true

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
