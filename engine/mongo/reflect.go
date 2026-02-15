package enginemongo

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// RelationKind describes the type of relationship between two entities.
type RelationKind = r3tag.RelationKind

const (
	// RelHasMany represents a one-to-many relationship.
	RelHasMany = r3tag.RelHasMany
	// RelBelongsTo represents a many-to-one relationship.
	RelBelongsTo = r3tag.RelBelongsTo
)

// RelationMeta holds metadata about a struct field that represents a relation.
type RelationMeta struct {
	FieldName  string       // Go struct field name (matched against PreloadSpec.Name)
	FieldIndex int          // struct field index for reflection-based assignment
	Kind       RelationKind // has-many or belongs-to
	FKField    string       // foreign key field name (BSON field name on the "many" side)
	TargetMeta StructMeta   // metadata for the related entity type
	TargetType reflect.Type // reflect.Type of the target entity (element type, not slice/ptr)
}

// StructMeta holds reflection-based metadata about a struct type T.
// It is the MongoDB equivalent of enginesql.StructMeta.
type StructMeta struct {
	CollectionName string   // e.g. "cities"
	Fields         []string // BSON field names in order, e.g. ["_id", "name", "country_name", ...]
	FieldIndices   []int    // corresponding struct field indices for each BSON field
	IDField        string   // primary key BSON field name (defaults to "_id")
	IDFieldIdx     int      // index into Fields/FieldIndices for the ID entry

	// SoftDeleteField is the BSON field name used for soft-delete (e.g. "deleted_at").
	// Empty string means soft-delete is not enabled.
	SoftDeleteField string

	// Relations holds metadata about related entities.
	Relations []RelationMeta
}

// GetStructMeta derives collection name and field info from a generic type T.
//
// Tag priority: `r3` tag first, `bson` tag as fallback, then `db` tag, then snake_case.
// Fields with tag value "-" (in any tag) are ignored.
func GetStructMeta[T any]() StructMeta {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, true)
}

// getStructMetaForType derives StructMeta from a reflect.Type (without parsing relations).
func getStructMetaForType(typ reflect.Type) StructMeta {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, false)
}

// buildStructMeta is the shared implementation for struct metadata extraction.
func buildStructMeta(typ reflect.Type, parseRelations bool) StructMeta {
	meta := StructMeta{
		CollectionName: r3utils.ToSnakeCasePlural(typ.Name()),
		IDField:        "_id",
		IDFieldIdx:     -1,
	}

	for i := range typ.NumField() {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		// Detect relation fields by Go type.
		if r3utils.IsRelationType(field.Type) {
			if parseRelations {
				if rel, ok := buildRelationMeta(field, i); ok {
					meta.Relations = append(meta.Relations, rel)
				}
			}
			continue
		}

		// Parse field tag: r3 -> bson -> db -> snake_case
		bsonName, isPK, isSoftDelete, skip := parseMongoFieldTag(field)
		if skip {
			continue
		}

		meta.Fields = append(meta.Fields, bsonName)
		meta.FieldIndices = append(meta.FieldIndices, i)

		if isPK || bsonName == "_id" {
			meta.IDField = bsonName
			meta.IDFieldIdx = len(meta.FieldIndices) - 1
		}

		if isSoftDelete {
			meta.SoftDeleteField = bsonName
		}
	}

	return meta
}

// parseMongoFieldTag reads field metadata from struct tags.
// Priority: `r3` tag first, `bson` tag as fallback, `db` tag as second fallback.
// Returns (bsonFieldName, isPK, isSoftDelete, skip).
func parseMongoFieldTag(field reflect.StructField) (string, bool, bool, bool) {
	r3Raw := field.Tag.Get("r3")
	bsonRaw := field.Tag.Get("bson")
	dbRaw := field.Tag.Get("db")

	// Check for skip
	if r3Raw == "-" || bsonRaw == "-" || dbRaw == "-" {
		return "", false, false, true
	}

	// Check for relation tags in r3 (skip those)
	if strings.HasPrefix(r3Raw, "rel:") || strings.Contains(r3Raw, ",rel:") {
		return "", false, false, true
	}

	// Parse r3 tag first
	tag := r3tag.ParseColumnTag(field)
	if tag.Skip {
		return "", false, false, true
	}

	// If r3 tag gave us a column name, use it
	bsonName := tag.Column
	isPK := tag.IsPK
	isSoftDelete := tag.SoftDelete

	// Override column name with bson tag if r3 didn't provide a specific name
	// and bson tag is available
	if bsonRaw != "" && bsonRaw != "-" {
		bsonParts := strings.Split(bsonRaw, ",")
		bsonTagName := strings.TrimSpace(bsonParts[0])
		if bsonTagName != "" {
			// If the r3 tag only set flags (pk, soft_delete) but no explicit name,
			// prefer the bson tag name
			if r3Raw == "" || isR3OnlyFlags(r3Raw) {
				bsonName = bsonTagName
			}
		}
	}

	// Map "id" to "_id" for MongoDB convention
	if bsonName == "id" && isPK {
		bsonName = "_id"
	}

	return bsonName, isPK, isSoftDelete, false
}

// isR3OnlyFlags returns true if the r3 tag only contains known flags (no column name).
func isR3OnlyFlags(raw string) bool {
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "pk", "soft_delete":
			continue
		default:
			if strings.HasPrefix(p, "rel:") || strings.HasPrefix(p, "fk:") || strings.HasPrefix(p, "table:") {
				continue
			}
			return false
		}
	}
	return true
}

// buildRelationMeta parses the r3 tag on a relation field.
func buildRelationMeta(field reflect.StructField, fieldIndex int) (RelationMeta, bool) {
	tag, ok := r3tag.ParseRelationTag(field)
	if !ok {
		return RelationMeta{}, false
	}

	targetType := r3utils.ResolveElementType(field.Type)
	targetMeta := getStructMetaForType(targetType)
	if tag.TableName != "" {
		targetMeta.CollectionName = tag.TableName
	}

	return RelationMeta{
		FieldName:  field.Name,
		FieldIndex: fieldIndex,
		Kind:       tag.Kind,
		FKField:    tag.FKColumn,
		TargetMeta: targetMeta,
		TargetType: targetType,
	}, true
}

// --------------------------------------------------------------------------
// StructMeta methods
// --------------------------------------------------------------------------

// NonIDFields returns all BSON field names except the ID field.
func (m *StructMeta) NonIDFields() []string {
	var fields []string
	for _, f := range m.Fields {
		if f != m.IDField {
			fields = append(fields, f)
		}
	}
	return fields
}

// FieldValues extracts the BSON field values from a struct in the same order as Fields.
func (m *StructMeta) FieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	vals := make([]any, len(m.FieldIndices))
	for i, idx := range m.FieldIndices {
		vals[i] = v.Field(idx).Interface()
	}
	return vals
}

// NonIDFieldValues extracts field values excluding the ID, for insert operations.
func (m *StructMeta) NonIDFieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	var vals []any
	for i, idx := range m.FieldIndices {
		if m.Fields[i] != m.IDField {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// IDValue extracts the ID field value from an entity.
func (m *StructMeta) IDValue(entity any) any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if m.IDFieldIdx >= 0 && m.IDFieldIdx < len(m.FieldIndices) {
		return v.Field(m.FieldIndices[m.IDFieldIdx]).Interface()
	}
	return nil
}

// SetIDValue sets the ID field value on an entity (via pointer).
func (m *StructMeta) SetIDValue(entityPtr any, val any) {
	v := reflect.ValueOf(entityPtr).Elem()
	if m.IDFieldIdx >= 0 && m.IDFieldIdx < len(m.FieldIndices) {
		idField := v.Field(m.FieldIndices[m.IDFieldIdx])
		idField.Set(reflect.ValueOf(val).Convert(idField.Type()))
	}
}

// FieldValuesForFields extracts field values from an entity for the given BSON field names.
func (m *StructMeta) FieldValuesForFields(entity any, fields []string) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	fieldToIdx := make(map[string]int, len(m.Fields))
	for i, f := range m.Fields {
		fieldToIdx[f] = m.FieldIndices[i]
	}

	vals := make([]any, 0, len(fields))
	for _, f := range fields {
		if idx, ok := fieldToIdx[f]; ok {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// ValidatePatchFields checks that the given field names are valid for a Patch operation.
func (m *StructMeta) ValidatePatchFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, r3.ErrNoPatchFields
	}

	known := make(map[string]bool, len(m.Fields))
	for _, f := range m.Fields {
		known[f] = true
	}

	for _, f := range fields {
		if !known[f] {
			return nil, fmt.Errorf("%w: %q does not exist", r3.ErrInvalidPatchField, f)
		}
		if f == m.IDField {
			return nil, fmt.Errorf("%w: %q is the ID field", r3.ErrInvalidPatchField, f)
		}
		if m.SoftDeleteField != "" && f == m.SoftDeleteField {
			return nil, fmt.Errorf("%w: %q is the soft-delete field", r3.ErrInvalidPatchField, f)
		}
	}

	return fields, nil
}

// ToBSONDoc converts an entity to a bson.D document using the struct metadata.
// This is used for insert/update operations.
func (m *StructMeta) ToBSONDoc(entity any, includeID bool) map[string]any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	doc := make(map[string]any, len(m.Fields))
	for i, idx := range m.FieldIndices {
		fieldName := m.Fields[i]
		if !includeID && fieldName == m.IDField {
			continue
		}
		doc[fieldName] = v.Field(idx).Interface()
	}
	return doc
}

// SetFieldsFromMap sets struct fields from a map of BSON field name -> value.
func (m *StructMeta) SetFieldsFromMap(entityPtr any, data map[string]any) {
	v := reflect.ValueOf(entityPtr).Elem()

	fieldToIdx := make(map[string]int, len(m.Fields))
	for i, f := range m.Fields {
		fieldToIdx[f] = m.FieldIndices[i]
	}

	for key, val := range data {
		if idx, ok := fieldToIdx[key]; ok {
			field := v.Field(idx)
			if val != nil {
				field.Set(reflect.ValueOf(val).Convert(field.Type()))
			}
		}
	}
}
