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

	// UsesOnConflictClause selects the upsert dialect. When true the backend
	// speaks Postgres/SQLite `ON CONFLICT (cols) DO UPDATE SET ...` (the conflict
	// target is explicit and EXCLUDED refers to the would-be-inserted row). When
	// false it speaks MySQL `ON DUPLICATE KEY UPDATE col = VALUES(col)` (the
	// conflict target is any unique key, so the columns are ignored).
	UsesOnConflictClause bool
}

// Pre-defined flavors for common databases.
var (
	FlavorPostgres = Flavor{
		Placeholder:          PlaceholderDollar,
		SupportsRETURNING:    true,
		TimestampFunc:        "NOW()",
		UsesOnConflictClause: true,
	}
	FlavorSQLite = Flavor{
		Placeholder:          PlaceholderQuestion,
		SupportsRETURNING:    true,
		TimestampFunc:        "datetime('now')",
		UsesOnConflictClause: true,
	}
	FlavorMySQL = Flavor{
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

// UpsertClause builds the trailing on-conflict clause of an upsert INSERT for
// this flavor: `ON CONFLICT (target) DO UPDATE SET col = EXCLUDED.col, ...` for
// Postgres/SQLite, `ON DUPLICATE KEY UPDATE col = VALUES(col), ...` for MySQL.
// conflictCols is the conflict target (ignored by MySQL, which keys off any
// unique index). An empty updateCols yields DO NOTHING (a self-assign no-op on
// MySQL) so a conflict leaves the existing row untouched.
func (f Flavor) UpsertClause(conflictCols, updateCols []string) string {
	if f.UsesOnConflictClause {
		target := strings.Join(conflictCols, ", ")
		if len(updateCols) == 0 {
			return fmt.Sprintf("ON CONFLICT (%s) DO NOTHING", target)
		}
		sets := make([]string, len(updateCols))
		for i, c := range updateCols {
			sets[i] = fmt.Sprintf("%s = EXCLUDED.%s", c, c)
		}
		return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s", target, strings.Join(sets, ", "))
	}

	// MySQL: the conflict target is implicit (any unique key). With no columns to
	// update, self-assign the first conflict column so the statement still parses
	// and behaves as a no-op on duplicate.
	if len(updateCols) == 0 {
		if len(conflictCols) == 0 {
			return ""
		}
		c := conflictCols[0]
		return fmt.Sprintf("ON DUPLICATE KEY UPDATE %s = %s", c, c)
	}
	sets := make([]string, len(updateCols))
	for i, c := range updateCols {
		sets[i] = fmt.Sprintf("%s = VALUES(%s)", c, c)
	}
	return "ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", ")
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
