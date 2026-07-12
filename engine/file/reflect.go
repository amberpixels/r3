package enginefile

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// timeType is used to distinguish time.Time from other structs during reflection.
var timeType = reflect.TypeFor[time.Time]()

// StructMeta holds reflection-based metadata about a struct type T for file storage.
type StructMeta struct {
	// ResourceName is the derived collection/resource name (e.g. "cities").
	// Used as the filename stem or directory name.
	ResourceName string

	// Fields holds the storage field names in order (derived from json/yaml/r3/db tags).
	Fields []string
	// FieldIndices holds the corresponding struct field indices for each field.
	FieldIndices []int

	// PKField is the primary key field name (defaults to "id").
	PKField string
	// PKFieldIdx is the index into Fields/FieldIndices for the PK entry.
	PKFieldIdx int

	// SoftDeleteField is the field name used for soft-delete (e.g. "deleted_at").
	// Empty string means soft-delete is not enabled.
	SoftDeleteField string
}

// GetStructMeta derives the resource name and field info for T. Field-name
// priority: `r3`, then `json`, then `yaml`, then `db`, then snake_case.
func GetStructMeta[T any]() StructMeta {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ)
}

// buildStructMeta is the shared implementation for struct metadata extraction.
func buildStructMeta(typ reflect.Type) StructMeta {
	meta := StructMeta{
		ResourceName: r3utils.ToSnakeCasePlural(typ.Name()),
		PKField:      "id",
		PKFieldIdx:   -1,
	}

	for i := range typ.NumField() {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		// Relation-type fields aren't stored (see isRelationType).
		if isRelationType(field.Type) {
			continue
		}

		fieldName, isPK, isSoftDelete, skip := parseFileFieldTag(field)
		if skip {
			continue
		}

		meta.Fields = append(meta.Fields, fieldName)
		meta.FieldIndices = append(meta.FieldIndices, i)

		if isPK || fieldName == "id" {
			meta.PKField = fieldName
			meta.PKFieldIdx = len(meta.FieldIndices) - 1
		}

		if isSoftDelete {
			meta.SoftDeleteField = fieldName
		}
	}

	return meta
}

// parseFileFieldTag reads a field's name and flags from its struct tags.
// Returns (fieldName, isPK, isSoftDelete, skip).
func parseFileFieldTag(field reflect.StructField) (string, bool, bool, bool) {
	r3Raw := field.Tag.Get("r3")
	jsonRaw := field.Tag.Get("json")
	yamlRaw := field.Tag.Get("yaml")

	if r3Raw == "-" || jsonRaw == "-" || yamlRaw == "-" {
		return "", false, false, true
	}

	// Relation fields are handled elsewhere, never stored.
	if strings.HasPrefix(r3Raw, "rel:") || strings.Contains(r3Raw, ",rel:") {
		return "", false, false, true
	}

	// r3 tag: PK / soft-delete flags and an optional column name.
	tag := r3tag.ParseColumnTag(field)
	if tag.Skip {
		return "", false, false, true
	}

	fieldName := tag.Column
	isPK := tag.IsPK
	isSoftDelete := tag.SoftDelete

	// No explicit name in r3 (flags only): fall back to json/yaml.
	if r3Raw == "" || isOnlyFlags(r3Raw) {
		if name := extractTagName(jsonRaw); name != "" {
			fieldName = name
		} else if name := extractTagName(yamlRaw); name != "" {
			fieldName = name
		}
	}

	return fieldName, isPK, isSoftDelete, false
}

// extractTagName extracts the field name from a struct tag value (first part before comma).
// Returns empty string if the tag is empty, "-", or only contains options.
func extractTagName(raw string) string {
	if raw == "" || raw == "-" {
		return ""
	}
	name, _, _ := strings.Cut(raw, ",")
	name = strings.TrimSpace(name)
	if name == "" || name == "-" {
		return ""
	}
	return name
}

// isOnlyFlags returns true if the r3 tag only contains known flags (no column name).
func isOnlyFlags(raw string) bool {
	for p := range strings.SplitSeq(raw, ",") {
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

// isRelationType returns true if the Go type represents a relation field
// (slice, map, pointer-to-struct, or struct - except time.Time).
func isRelationType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Map:
		return true
	case reflect.Pointer:
		return t.Elem().Kind() == reflect.Struct && t.Elem() != timeType
	case reflect.Struct:
		return t != timeType
	default:
		return false
	}
}

// PKValue extracts the primary key value from an entity.
func (m *StructMeta) PKValue(entity any) any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if m.PKFieldIdx >= 0 && m.PKFieldIdx < len(m.FieldIndices) {
		return v.Field(m.FieldIndices[m.PKFieldIdx]).Interface()
	}
	return nil
}

// SetPKValue sets the primary key value on an entity (via pointer).
func (m *StructMeta) SetPKValue(entityPtr any, val any) {
	v := reflect.ValueOf(entityPtr)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if m.PKFieldIdx >= 0 && m.PKFieldIdx < len(m.FieldIndices) {
		pkField := v.Field(m.FieldIndices[m.PKFieldIdx])
		pkField.Set(reflect.ValueOf(val).Convert(pkField.Type()))
	}
}

// GetFieldValue extracts a field value from an entity by field name (storage name).
func (m *StructMeta) GetFieldValue(entity any, fieldName string) (any, bool) {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	for i, f := range m.Fields {
		if f == fieldName {
			return v.Field(m.FieldIndices[i]).Interface(), true
		}
	}
	return nil, false
}

// SetFieldValue sets a field value on an entity (via pointer) by field name.
func (m *StructMeta) SetFieldValue(entityPtr any, fieldName string, val any) bool {
	v := reflect.ValueOf(entityPtr)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	for i, f := range m.Fields {
		if f == fieldName {
			field := v.Field(m.FieldIndices[i])
			field.Set(reflect.ValueOf(val).Convert(field.Type()))
			return true
		}
	}
	return false
}

// CopyFieldValues copies the specified fields from src to dst (both must be pointers).
func (m *StructMeta) CopyFieldValues(dst any, src any, fieldNames []string) {
	dstV := reflect.ValueOf(dst)
	if dstV.Kind() == reflect.Pointer {
		dstV = dstV.Elem()
	}
	srcV := reflect.ValueOf(src)
	if srcV.Kind() == reflect.Pointer {
		srcV = srcV.Elem()
	}

	fieldSet := make(map[string]struct{}, len(fieldNames))
	for _, f := range fieldNames {
		fieldSet[f] = struct{}{}
	}

	for i, f := range m.Fields {
		if _, ok := fieldSet[f]; ok {
			idx := m.FieldIndices[i]
			dstV.Field(idx).Set(srcV.Field(idx))
		}
	}
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
		if f == m.PKField {
			return nil, fmt.Errorf("%w: %q is the primary key field", r3.ErrInvalidPatchField, f)
		}
		if m.SoftDeleteField != "" && f == m.SoftDeleteField {
			return nil, fmt.Errorf("%w: %q is the soft-delete field", r3.ErrInvalidPatchField, f)
		}
	}

	return fields, nil
}

// SoftDeleteFieldIdx returns the struct field index for the soft-delete field, or -1.
func (m *StructMeta) SoftDeleteFieldIdx() int {
	if m.SoftDeleteField == "" {
		return -1
	}
	for i, f := range m.Fields {
		if f == m.SoftDeleteField {
			return m.FieldIndices[i]
		}
	}
	return -1
}
