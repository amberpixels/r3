package r3sql

import (
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// QuoteIdentifier double-quotes a SQL identifier, escaping embedded quotes by
// doubling them (ANSI standard). Safe for PostgreSQL, SQLite, and MySQL: the
// major ORMs all accept double-quoted identifiers.
//
//	QuoteIdentifier("id")          => `"id"`
//	QuoteIdentifier(`col"name`)    => `"col""name"` (escaped)
func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// QuoteDottedIdentifier quotes each segment of a dotted path and rejoins with
// dots: "user.name" => `"user"."name"`.
func QuoteDottedIdentifier(path string) string {
	parts := strings.Split(path, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = QuoteIdentifier(part)
	}
	return strings.Join(quoted, ".")
}

// unquoteSQLColumn reverses QuoteDottedIdentifier, recovering the original field
// name. An unquoted segment passes through unchanged, so a hand-built (unquoted)
// sort string still parses. `"user"."name"` => `user.name`.
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

// SafeColumnExpr validates a FieldSpec and returns its quoted SQL expression -
// the SQL-injection defense used by FilterToSQL and SortToSQL. Two layers:
// r3.ValidateIdentifier restricts the name to safe characters, then each
// segment is double-quoted. Errors on an invalid name.
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

// safeJoinTable validates and quotes a table name (a single segment, no dots)
// extracted from a dotted field path.
func safeJoinTable(tableName string) (SQLColumn, error) {
	if err := r3.ValidateIdentifier(tableName); err != nil {
		return "", fmt.Errorf("%w: join table %q", err, tableName)
	}
	return SQLColumn(QuoteIdentifier(tableName)), nil
}
