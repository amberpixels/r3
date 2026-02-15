// Package r3tag provides struct tag parsing for r3 entities.
//
// It handles the `r3` tag (primary) with `db` tag as fallback for column mapping.
//
// Column tag syntax (r3 or db):
//
//	r3:"column_name"
//	r3:"column_name,pk"
//	r3:"column_name,pk,soft_delete"
//	r3:"-"               // skip this field
//	r3:"soft_delete"      // column name derived from field name
//	db:"column_name,pk"  // fallback when no r3 tag
//
// Relation tag syntax (r3 only):
//
//	r3:"rel:has-many,fk:city_id"
//	r3:"rel:belongs-to,fk:city_id"
//	r3:"rel:has-many,fk:city_id,table:translations"
package r3tag

import (
	"reflect"
	"strings"

	"github.com/amberpixels/r3/internal/utils"
)

// ColumnTag holds parsed column-level tag info for a single struct field.
type ColumnTag struct {
	Column     string // resolved column name
	IsPK       bool   // field is the primary key
	Skip       bool   // field should be skipped (tag value is "-")
	SoftDelete bool   // r3:"soft_delete"
}

// ParseColumnTag reads column metadata from struct tags.
// Priority: `r3` tag first, `db` tag as fallback.
// If neither tag provides a column name, snake_case of the field name is used.
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

	// Final fallback: derive column name from Go field name.
	if tag.Column == "" {
		tag.Column = r3utils.ToSnakeCase(field.Name)
	}

	return tag
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
	case "pk", "soft_delete":
		return true
	}
	return strings.HasPrefix(s, "rel:") ||
		strings.HasPrefix(s, "fk:") ||
		strings.HasPrefix(s, "table:")
}

// applyFlag applies a known tag flag to a ColumnTag.
func applyFlag(tag *ColumnTag, flag string) {
	switch flag {
	case "pk":
		tag.IsPK = true
	case "soft_delete":
		tag.SoftDelete = true
	}
}

// RelationKind describes the type of relationship between two entities.
type RelationKind int

const (
	// RelHasMany represents a one-to-many relationship.
	RelHasMany RelationKind = iota
	// RelBelongsTo represents a many-to-one relationship.
	RelBelongsTo
)

// RelationTag holds parsed relation tag info.
type RelationTag struct {
	Kind      RelationKind
	FKColumn  string // foreign key column name
	TableName string // explicit table name override (optional)
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
		case strings.HasPrefix(part, "fk:"):
			tag.FKColumn = strings.TrimPrefix(part, "fk:")
		case strings.HasPrefix(part, "table:"):
			tag.TableName = strings.TrimPrefix(part, "table:")
		}
	}

	if !hasRel || tag.FKColumn == "" {
		return RelationTag{}, false
	}

	return tag, true
}
