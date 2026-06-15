package enginesql

import (
	"fmt"
	"strings"
)

// PlaceholderStyle controls how SQL parameter placeholders are generated.
type PlaceholderStyle int

const (
	// PlaceholderQuestion uses `?` placeholders (MySQL, SQLite).
	PlaceholderQuestion PlaceholderStyle = iota
	// PlaceholderDollar uses `$1, $2, ...` numbered placeholders (PostgreSQL).
	PlaceholderDollar
)

// Flavor describes the SQL dialect differences between database backends.
// It is passed to NewBaseCRUD to configure SQL generation.
type Flavor struct {
	// Placeholder controls the parameter placeholder style (? vs $N).
	Placeholder PlaceholderStyle

	// SupportsRETURNING indicates whether the database supports the RETURNING clause
	// in INSERT statements (PostgreSQL, SQLite 3.35+).
	// When false, Create() uses INSERT + last-insert-id + SELECT.
	SupportsRETURNING bool

	// TimestampFunc is the SQL expression for the current timestamp.
	// Used by soft-delete to set the deleted_at column.
	// Examples: "NOW()" (PostgreSQL), "CURRENT_TIMESTAMP" (MySQL), "datetime('now')" (SQLite).
	TimestampFunc string

	// IdentifierQuote is the character this backend uses to quote SQL identifiers.
	// The dialect layer emits ANSI double quotes (`"col"`); the engine rewrites
	// them to this character when assembling the final query (see QuoteIdentifiers).
	// Empty means ANSI double quotes (Postgres, SQLite). MySQL needs backticks,
	// because without ANSI_QUOTES mode it reads `"col"` as a string literal — which
	// silently breaks every filter and sort built from the dialect.
	IdentifierQuote string
}

// Pre-defined flavors for common databases.
var (
	FlavorPostgres = Flavor{Placeholder: PlaceholderDollar, SupportsRETURNING: true, TimestampFunc: "NOW()"}
	FlavorSQLite   = Flavor{Placeholder: PlaceholderQuestion, SupportsRETURNING: true, TimestampFunc: "datetime('now')"}
	FlavorMySQL    = Flavor{
		Placeholder:       PlaceholderQuestion,
		SupportsRETURNING: false,
		TimestampFunc:     "CURRENT_TIMESTAMP",
		IdentifierQuote:   "`",
	}
)

// Placeholders generates a placeholder string for count parameters.
// For PlaceholderDollar starting from startIdx: "$1, $2, $3"
// For PlaceholderQuestion: "?, ?, ?".
func (f Flavor) Placeholders(count int, startIdx int) string {
	parts := make([]string, count)
	switch f.Placeholder {
	case PlaceholderQuestion:
		for i := range parts {
			parts[i] = "?"
		}
	case PlaceholderDollar:
		for i := range parts {
			parts[i] = fmt.Sprintf("$%d", startIdx+i)
		}
	}
	return strings.Join(parts, ", ")
}

// SetExprs generates "col1 = $1, col2 = $2, ..." or "col1 = ?, col2 = ?, ..."
// for UPDATE SET clauses.
func (f Flavor) SetExprs(cols []string, startIdx int) string {
	parts := make([]string, len(cols))
	switch f.Placeholder {
	case PlaceholderQuestion:
		for i, col := range cols {
			parts[i] = col + " = ?"
		}
	case PlaceholderDollar:
		for i, col := range cols {
			parts[i] = fmt.Sprintf("%s = $%d", col, startIdx+i)
		}
	}
	return strings.Join(parts, ", ")
}

// WhereEq generates "column = $N" or "column = ?" depending on the placeholder style.
// nextIdx is the 1-based parameter index (used only for dollar-style).
func (f Flavor) WhereEq(column string, nextIdx int) string {
	switch f.Placeholder {
	case PlaceholderQuestion:
		return column + " = ?"
	case PlaceholderDollar:
		return fmt.Sprintf("%s = $%d", column, nextIdx)
	}
	return column + " = ?" // fallback
}

// QuoteIdentifiers rewrites the ANSI double-quoted identifiers emitted by the
// dialect layer (`"col"`, `"tbl"."col"`) to this flavor's identifier quote. It is
// a no-op for flavors that use ANSI double quotes (Postgres, SQLite).
//
// It is safe to apply to a fully assembled query: values are always bound as
// parameters (`?` / `$N`), never inlined, so the only double quotes in generated
// SQL are identifier quotes. The dialect escapes an embedded quote by doubling it
// (`""`), and a plain ReplaceAll maps that to the doubled target quote — which is
// also the correct escape — so escaping is preserved. (In practice identifiers are
// validated to contain no quotes, so escaping never actually occurs.)
func (f Flavor) QuoteIdentifiers(sql string) string {
	if f.IdentifierQuote == "" || f.IdentifierQuote == `"` {
		return sql
	}
	return strings.ReplaceAll(sql, `"`, f.IdentifierQuote)
}

// ConvertPlaceholders converts `?` placeholders in clause to `$1, $2, ...`
// starting from startIdx. Returns the converted string and the next available index.
// This is a no-op when the flavor uses `?` placeholders.
func (f Flavor) ConvertPlaceholders(clause string, startIdx int) (string, int) {
	if f.Placeholder == PlaceholderQuestion {
		// Count ?'s to advance the index correctly
		count := strings.Count(clause, "?")
		return clause, startIdx + count
	}
	var b strings.Builder
	idx := startIdx
	for i := range len(clause) {
		if clause[i] == '?' {
			fmt.Fprintf(&b, "$%d", idx)
			idx++
		} else {
			b.WriteByte(clause[i])
		}
	}
	return b.String(), idx
}
