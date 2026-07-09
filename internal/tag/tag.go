// Package r3tag provides struct tag parsing for r3 entities.
//
// It handles the `r3` tag (primary) with `db` tag as fallback for column
// mapping, and finally the `gorm` tag (column:/primaryKey) so gorm-tagged
// models resolve to the same physical columns the GORM driver binds.
//
// Column tag syntax (r3 or db):
//
//	r3:"column_name"
//	r3:"column_name,pk"
//	r3:"column_name,pk,soft_delete"
//	r3:"-"               // skip this field
//	r3:"soft_delete"      // column name derived from field name
//	db:"column_name,pk"  // fallback when no r3 tag
//	gorm:"column:name"   // fallback when neither r3 nor db names the column
//
// Relation tag syntax (r3 only):
//
//	r3:"rel:has-many,fk:city_id"
//	r3:"rel:belongs-to,fk:city_id"
//	r3:"rel:has-many,fk:city_id,table:translations"
//
// Capability flags (additive — they tighten the permissive defaults, never widen):
//
//	r3:"secret_token,no-filter,no-sort,no-output" // hidden from filter/sort/select
//	r3:"population,readonly"                        // not creatable, not mutable
//	r3:"slug,immutable"                             // creatable once, not mutable
//	r3:"status,enum:draft|planned|published"        // enum type + allowed values
package r3tag

import (
	"reflect"
	"strings"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// ColumnTag holds parsed column-level tag info for a single struct field.
type ColumnTag struct {
	Column     string // resolved column name
	IsPK       bool   // field is the primary key
	Skip       bool   // field should be skipped (tag value is "-")
	SoftDelete bool   // r3:"soft_delete"

	// Capability flags. They are additive and only ever tighten the schema's
	// permissive defaults (see r3.SchemaOf) — never widen them.
	NoFilter  bool     // r3:"...,no-filter"  — not allowed in Query.Filters
	NoSort    bool     // r3:"...,no-sort"    — not allowed in Query.Sorts
	NoOutput  bool     // r3:"...,no-output"  — hidden from SELECT and serialized output
	ReadOnly  bool     // r3:"...,readonly"   — not creatable and not mutable
	Immutable bool     // r3:"...,immutable"  — creatable once, then not mutable
	Enum      []string // r3:"...,enum:a|b|c" — enum data type with the allowed values
	Codec     string   // r3:"...,codec:name" — value codec name (resolved in schema derivation)
}

// ParseColumnTag reads column metadata from struct tags.
// Priority: `r3` tag first, `db` tag as fallback, then the `gorm` tag's
// column/primaryKey info so gorm-tagged models derive the same physical
// columns the GORM driver binds.
// If no tag provides a column name, snake_case of the field name is used.
func ParseColumnTag(field reflect.StructField) ColumnTag {
	r3Raw := field.Tag.Get("r3")
	dbRaw := field.Tag.Get("db")

	// Use r3 tag if present, otherwise fall back to db tag.
	raw := r3Raw
	if raw == "" {
		raw = dbRaw
	}

	if raw == "-" {
		return ColumnTag{Skip: true}
	}

	var tag ColumnTag
	if raw != "" {
		tag = parseRawColumnTag(raw)
	}

	// If r3 tag was used but produced no column name (e.g. r3:"soft_delete"),
	// try to get column name from the db tag.
	if tag.Column == "" && dbRaw != "" && dbRaw != "-" {
		dbTag := parseRawColumnTag(dbRaw)
		if dbTag.Column != "" {
			tag.Column = dbTag.Column
		}
		if dbTag.IsPK {
			tag.IsPK = true
		}
	}

	// Fall back to the gorm tag for gaps r3/db left: column name (gorm:"column:x")
	// and primary key. A gorm:"-" only skips the field when no r3/db tag claims it.
	if gormRaw := field.Tag.Get("gorm"); gormRaw != "" {
		col, isPK, skip := parseGormColumnTag(gormRaw)
		if skip && raw == "" {
			return ColumnTag{Skip: true}
		}
		if tag.Column == "" && col != "" {
			tag.Column = col
		}
		if isPK {
			tag.IsPK = true
		}
	}

	// Final fallback: derive column name from Go field name.
	if tag.Column == "" {
		tag.Column = r3utils.ToSnakeCase(field.Name)
	}

	return tag
}

// parseGormColumnTag extracts the column name and primary-key flag from a
// `gorm` struct tag (semicolon-separated, e.g. gorm:"column:venue_id;primaryKey").
// Only the parts relevant to column mapping are read; gorm tag keys are
// case-insensitive.
func parseGormColumnTag(raw string) (string, bool, bool) {
	if raw == "-" {
		return "", false, true
	}
	var column string
	var isPK bool
	for part := range strings.SplitSeq(raw, ";") {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "column:"):
			column = strings.TrimSpace(part[len("column:"):])
		case lower == "primarykey" || lower == "primary_key":
			isPK = true
		}
	}
	return column, isPK, false
}

// parseRawColumnTag parses a single raw tag value (from either r3 or db tag).
func parseRawColumnTag(raw string) ColumnTag {
	var tag ColumnTag
	parts := strings.Split(raw, ",")

	first := strings.TrimSpace(parts[0])

	// Check if the first part is a known keyword rather than a column name.
	if isKnownKeyword(first) {
		// It's a flag, not a column name — process it as a flag.
		applyFlag(&tag, first)
	} else {
		tag.Column = first
	}

	for _, part := range parts[1:] {
		applyFlag(&tag, strings.TrimSpace(part))
	}

	return tag
}

// isKnownKeyword returns true if the given string is a recognized r3 tag keyword
// that should not be treated as a column name.
func isKnownKeyword(s string) bool {
	switch s {
	case "pk", "soft_delete", "owned",
		"no-filter", "no-sort", "no-output", "readonly", "immutable":
		return true
	}
	return strings.HasPrefix(s, "rel:") ||
		strings.HasPrefix(s, "fk:") ||
		strings.HasPrefix(s, "ref:") ||
		strings.HasPrefix(s, "join:") ||
		strings.HasPrefix(s, "table:") ||
		strings.HasPrefix(s, "enum:") ||
		strings.HasPrefix(s, "codec:")
}

// applyFlag applies a known tag flag to a ColumnTag.
func applyFlag(tag *ColumnTag, flag string) {
	switch flag {
	case "pk":
		tag.IsPK = true
	case "soft_delete":
		tag.SoftDelete = true
	case "no-filter":
		tag.NoFilter = true
	case "no-sort":
		tag.NoSort = true
	case "no-output":
		tag.NoOutput = true
	case "readonly":
		tag.ReadOnly = true
	case "immutable":
		tag.Immutable = true
	default:
		if values, ok := strings.CutPrefix(flag, "enum:"); ok {
			tag.Enum = parseEnumValues(values)
		}
		if name, ok := strings.CutPrefix(flag, "codec:"); ok {
			tag.Codec = strings.TrimSpace(name)
		}
	}
}

// parseEnumValues splits a pipe-separated enum value list ("a|b|c") into a
// trimmed slice, dropping empty entries.
func parseEnumValues(raw string) []string {
	var out []string
	for v := range strings.SplitSeq(raw, "|") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// RelationKind describes the type of relationship between two entities.
type RelationKind int

const (
	// RelHasMany represents a one-to-many relationship.
	RelHasMany RelationKind = iota
	// RelBelongsTo represents a many-to-one relationship.
	RelBelongsTo
	// RelManyToMany represents a many-to-many relationship via a join table.
	RelManyToMany
)

// RelationTag holds parsed relation tag info.
type RelationTag struct {
	Kind      RelationKind
	FKColumn  string // foreign key column name (or left FK for M2M)
	RefColumn string // right FK column for M2M join table
	TableName string // explicit table name override (optional)
	JoinTable string // join table name for M2M relations
	Owned     bool   // if true, children are lifecycle-bound to parent (delete orphans on update)
}

// ParseRelationTag parses the `r3` struct tag on a relation field.
// Returns a RelationTag and true if the tag declares a valid relation.
func ParseRelationTag(field reflect.StructField) (RelationTag, bool) {
	r3Raw := field.Tag.Get("r3")
	if r3Raw == "" {
		return RelationTag{}, false
	}

	parts := strings.Split(r3Raw, ",")
	var tag RelationTag
	var hasRel bool

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "rel:has-many":
			tag.Kind = RelHasMany
			hasRel = true
		case part == "rel:belongs-to":
			tag.Kind = RelBelongsTo
			hasRel = true
		case part == "rel:many-to-many":
			tag.Kind = RelManyToMany
			hasRel = true
		case part == "owned":
			tag.Owned = true
		case strings.HasPrefix(part, "fk:"):
			tag.FKColumn = strings.TrimPrefix(part, "fk:")
		case strings.HasPrefix(part, "ref:"):
			tag.RefColumn = strings.TrimPrefix(part, "ref:")
		case strings.HasPrefix(part, "join:"):
			tag.JoinTable = strings.TrimPrefix(part, "join:")
		case strings.HasPrefix(part, "table:"):
			tag.TableName = strings.TrimPrefix(part, "table:")
		}
	}

	if !hasRel || tag.FKColumn == "" {
		return RelationTag{}, false
	}

	return tag, true
}
