package r3sql

import (
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// QuoteIdentifier wraps a single SQL identifier in double quotes,
// escaping any embedded double quotes by doubling them (ANSI SQL standard).
//
// This produces identifiers safe for PostgreSQL, SQLite, and MySQL (in ANSI mode).
// MySQL's default backtick quoting also accepts double-quoted identifiers when
// ANSI_QUOTES mode is enabled, but all major ORMs (GORM, Bun, go-pg) already
// handle this correctly when receiving double-quoted identifiers.
//
// Examples:
//
//	QuoteIdentifier("id")          => `"id"`
//	QuoteIdentifier("user_name")   => `"user_name"`
//	QuoteIdentifier(`col"name`)    => `"col""name"` (escaped)
func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// QuoteDottedIdentifier quotes a dotted identifier path (e.g., "user.name")
// by quoting each segment individually and joining with dots.
//
// Examples:
//
//	QuoteDottedIdentifier("id")            => `"id"`
//	QuoteDottedIdentifier("user.name")     => `"user"."name"`
//	QuoteDottedIdentifier("a.b.c")         => `"a"."b"."c"`
func QuoteDottedIdentifier(path string) string {
	parts := strings.Split(path, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = QuoteIdentifier(part)
	}
	return strings.Join(quoted, ".")
}

// unquoteSQLColumn reverses QuoteDottedIdentifier: it strips the double-quote
// wrapping from each dotted segment and un-escapes doubled quotes, recovering the
// original field name. A segment that is not quoted is returned unchanged, so a
// hand-built (unquoted) sort string still parses.
//
// Examples:
//
//	unquoteSQLColumn(`"name"`)          => `name`
//	unquoteSQLColumn(`"user"."name"`)   => `user.name`
//	unquoteSQLColumn(`name`)            => `name`
func unquoteSQLColumn(s string) string {
	parts := strings.Split(s, ".")
	for i, part := range parts {
		if len(part) >= 2 && strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
			part = part[1 : len(part)-1]
			part = strings.ReplaceAll(part, `""`, `"`)
		}
		parts[i] = part
	}
	return strings.Join(parts, ".")
}

// SafeColumnExpr validates a FieldSpec as a safe SQL identifier and returns
// the quoted SQL expression. This is the primary function used by FilterToSQL
// and SortToSQL to prevent SQL injection via field names.
//
// It performs two layers of defense:
//  1. Validates that the field name contains only safe identifier characters
//     (letters, digits, underscores, dots) via r3.ValidateIdentifier.
//  2. Quotes each identifier segment with double quotes to prevent
//     any remaining edge cases.
//
// Returns an error if the field name is invalid.
func SafeColumnExpr(f *r3.FieldSpec) (string, error) {
	if f == nil {
		return "", fmt.Errorf("%w: nil field", r3.ErrInvalidIdentifier)
	}

	name := f.String()
	if err := r3.ValidateIdentifier(name); err != nil {
		return "", fmt.Errorf("%w: %q", err, name)
	}

	return QuoteDottedIdentifier(name), nil
}

// safeJoinTable validates and quotes a table name extracted from a dotted field path.
// It expects a single identifier segment (no dots).
func safeJoinTable(tableName string) (SQLColumn, error) {
	if err := r3.ValidateIdentifier(tableName); err != nil {
		return "", fmt.Errorf("%w: join table %q", err, tableName)
	}
	return SQLColumn(QuoteIdentifier(tableName)), nil
}
