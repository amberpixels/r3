package sqlbase

import (
	"reflect"
	"strings"
)

// StructMeta holds reflection-based metadata about a struct type T.
// It is used by BaseCRUD and BaseRaw to build SQL queries and scan results.
type StructMeta struct {
	TableName string   // e.g. "cities"
	Columns   []string // column names in order, e.g. ["id", "name", "country_name", ...]
	Fields    []int    // corresponding struct field indices for each column
	PKColumn  string   // primary key column name (defaults to "id")
	PKField   int      // index into Columns/Fields for the PK entry
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

		// Skip relations: slices, maps, pointer-to-struct, and struct fields (except time.Time).
		// Pointer-to-basic types (e.g. *string, *int64) are kept for nullable columns.
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

		// Parse the `db` tag
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
