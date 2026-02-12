package r3pq

import (
	"fmt"
	"reflect"
	"strings"
)

// structMeta holds reflection-based metadata about a struct type T.
type structMeta struct {
	tableName string   // e.g. "cities"
	columns   []string // column names in order, e.g. ["id", "name", "country_name", ...]
	fields    []int    // corresponding struct field indices for each column
	pkColumn  string   // primary key column name (defaults to "id")
	pkField   int      // struct field index for PK
}

// getStructMeta derives table name and column info from a generic type T.
// It looks for `db:"column_name"` struct tags. Fields without a `db` tag
// use the snake_case version of the field name. Fields with `db:"-"` are ignored.
// Pointer fields, slice fields, and struct fields (relations) are skipped.
func getStructMeta[T any]() structMeta {
	var t T
	typ := reflect.TypeOf(t)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	meta := structMeta{
		tableName: toSnakeCasePlural(typ.Name()),
		pkColumn:  "id",
		pkField:   -1,
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
		if fKind == reflect.Ptr {
			// Allow pointer-to-basic types (e.g. *string for nullable columns),
			// but skip pointer-to-struct (relations like *City).
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
			colName = toSnakeCase(field.Name)
		}

		meta.columns = append(meta.columns, colName)
		meta.fields = append(meta.fields, i)

		if isPK || colName == "id" {
			meta.pkColumn = colName
			meta.pkField = len(meta.fields) - 1
		}
	}

	return meta
}

// nonPKColumns returns all column names except the primary key.
func (m *structMeta) nonPKColumns() []string {
	var cols []string
	for _, c := range m.columns {
		if c != m.pkColumn {
			cols = append(cols, c)
		}
	}
	return cols
}

// fieldValues extracts the column values from a struct in the same order as meta.columns.
func (m *structMeta) fieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	vals := make([]any, len(m.fields))
	for i, idx := range m.fields {
		vals[i] = v.Field(idx).Interface()
	}
	return vals
}

// nonPKFieldValues extracts column values excluding the PK, for INSERT/UPDATE.
func (m *structMeta) nonPKFieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	var vals []any
	for i, idx := range m.fields {
		if m.columns[i] != m.pkColumn {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// scanDest returns a slice of pointers to the struct fields, suitable for sql.Row.Scan().
// It writes into the provided entity pointer.
func (m *structMeta) scanDest(entity any) []any {
	v := reflect.ValueOf(entity).Elem()
	dests := make([]any, len(m.fields))
	for i, idx := range m.fields {
		dests[i] = v.Field(idx).Addr().Interface()
	}
	return dests
}

// pkValue extracts the primary key value from an entity.
func (m *structMeta) pkValue(entity any) any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if m.pkField >= 0 && m.pkField < len(m.fields) {
		return v.Field(m.fields[m.pkField]).Interface()
	}
	return nil
}

// setPKValue sets the primary key value on an entity (via pointer).
func (m *structMeta) setPKValue(entityPtr any, val any) {
	v := reflect.ValueOf(entityPtr).Elem()
	if m.pkField >= 0 && m.pkField < len(m.fields) {
		pkField := v.Field(m.fields[m.pkField])
		pkField.Set(reflect.ValueOf(val).Convert(pkField.Type()))
	}
}

// --- String utilities ---

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			result.WriteRune(r + 32) // to lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// toSnakeCasePlural converts CamelCase to snake_case and applies naive pluralization.
func toSnakeCasePlural(s string) string {
	snake := toSnakeCase(s)

	if strings.HasSuffix(snake, "y") {
		return snake[:len(snake)-1] + "ies"
	}
	if strings.HasSuffix(snake, "s") || strings.HasSuffix(snake, "x") ||
		strings.HasSuffix(snake, "sh") || strings.HasSuffix(snake, "ch") {
		return snake + "es"
	}
	return snake + "s"
}

// convertPlaceholders converts `?` placeholders to `$1, $2, $3, ...` starting from startIdx.
// Returns the converted string and the next available index.
func convertPlaceholders(clause string, startIdx int) (string, int) {
	var b strings.Builder
	idx := startIdx
	for i := 0; i < len(clause); i++ {
		if clause[i] == '?' {
			b.WriteString(fmt.Sprintf("$%d", idx))
			idx++
		} else {
			b.WriteByte(clause[i])
		}
	}
	return b.String(), idx
}

// columnsString joins column names with commas.
func columnsString(cols []string) string {
	return strings.Join(cols, ", ")
}

// placeholders generates "$1, $2, ..., $n" starting from startIdx.
func placeholders(count int, startIdx int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", startIdx+i)
	}
	return strings.Join(parts, ", ")
}

// setExprs generates "col1 = $1, col2 = $2, ..." for UPDATE SET clause.
func setExprs(cols []string, startIdx int) string {
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = fmt.Sprintf("%s = $%d", col, startIdx+i)
	}
	return strings.Join(parts, ", ")
}
