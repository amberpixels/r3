package enginesql

import (
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// PlaceholderStyle controls how SQL parameter placeholders are generated.
type PlaceholderStyle int

const (
	// PlaceholderQuestion uses `?` placeholders (MySQL, SQLite).
	PlaceholderQuestion PlaceholderStyle = iota
	// PlaceholderDollar uses `$1, $2, ...` numbered placeholders (PostgreSQL).
	PlaceholderDollar
)

// Flavor describes the SQL dialect differences between backends, passed to
// NewBaseCRUD to configure SQL generation.
type Flavor struct {
	// Placeholder controls the parameter placeholder style (? vs $N).
	Placeholder PlaceholderStyle

	// SupportsRETURNING reports whether INSERT supports RETURNING (Postgres,
	// SQLite 3.35+). When false, Create() falls back to INSERT + last-insert-id +
	// SELECT.
	SupportsRETURNING bool

	// TimestampFunc is the current-timestamp SQL expression used by soft-delete to
	// set deleted_at: "NOW()" (Postgres), "CURRENT_TIMESTAMP" (MySQL),
	// "datetime('now')" (SQLite).
	TimestampFunc string

	// IdentifierQuote is this backend's identifier quote char; the engine rewrites
	// the dialect's ANSI double quotes to it (see QuoteIdentifiers). Empty = ANSI
	// double quotes (Postgres, SQLite). MySQL needs backticks: without ANSI_QUOTES
	// mode it reads `"col"` as a string literal, silently breaking every filter and
	// sort.
	IdentifierQuote string

	// UsesOnConflictClause selects the upsert dialect. True = Postgres/SQLite
	// `ON CONFLICT (cols) DO UPDATE SET ...` (explicit target; EXCLUDED is the
	// would-be-inserted row). False = MySQL `ON DUPLICATE KEY UPDATE col =
	// VALUES(col)` (target is any unique key, so cols are ignored).
	UsesOnConflictClause bool

	// BucketExpr renders a flavor-specific truncation of col (an already
	// ANSI-quoted time column reference) to a calendar unit, for a time-bucket
	// GROUP BY key. nil means the flavor has no bucket support, so a bucket query
	// errors loudly (see [Flavor.DateTruncExpr]) instead of silently returning
	// un-bucketed rows. Set on the predefined flavors; ISO-Monday weeks.
	BucketExpr func(col string, unit r3.BucketUnit) (string, error)
}

// DateTruncExpr renders the flavor's truncation of col to unit, or a loud
// [r3.ErrBucketNotSupported] when the flavor has no bucket support wired.
func (f Flavor) DateTruncExpr(col string, unit r3.BucketUnit) (string, error) {
	if f.BucketExpr == nil {
		return "", fmt.Errorf("%w: SQL flavor has no date-truncation hook", r3.ErrBucketNotSupported)
	}
	return f.BucketExpr(col, unit)
}

// postgresBucketExpr truncates via date_trunc; date_trunc('week') already starts
// on Monday (ISO-8601).
func postgresBucketExpr(col string, unit r3.BucketUnit) (string, error) {
	if !unit.Valid() {
		return "", fmt.Errorf("%w: unknown bucket unit %d", r3.ErrInvalidBucket, unit)
	}
	return fmt.Sprintf("date_trunc('%s', %s)", unit.String(), col), nil
}

// sqliteBucketExpr truncates via strftime/date, returning TEXT (the AggregateRow
// accessors parse it). The week form shifts back to Monday: %w is 0=Sunday..
// 6=Saturday, so days-since-Monday is (%w + 6) % 7.
func sqliteBucketExpr(col string, unit r3.BucketUnit) (string, error) {
	switch unit {
	case r3.BucketHour:
		return "strftime('%Y-%m-%d %H:00:00', " + col + ")", nil
	case r3.BucketDay:
		return "date(" + col + ")", nil
	case r3.BucketWeek:
		return "date(" + col + ", '-' || ((strftime('%w', " + col + ") + 6) % 7) || ' days')", nil
	case r3.BucketMonth:
		return "strftime('%Y-%m-01', " + col + ")", nil
	case r3.BucketYear:
		return "strftime('%Y-01-01', " + col + ")", nil
	default:
		return "", fmt.Errorf("%w: unknown bucket unit %d", r3.ErrInvalidBucket, unit)
	}
}

// mysqlBucketExpr truncates via DATE/DATE_FORMAT. WEEKDAY is 0=Monday, so
// subtracting WEEKDAY(col) days lands on the ISO-Monday week start.
func mysqlBucketExpr(col string, unit r3.BucketUnit) (string, error) {
	switch unit {
	case r3.BucketHour:
		return "DATE_FORMAT(" + col + ", '%Y-%m-%d %H:00:00')", nil
	case r3.BucketDay:
		return "DATE(" + col + ")", nil
	case r3.BucketWeek:
		return "DATE(DATE_SUB(" + col + ", INTERVAL WEEKDAY(" + col + ") DAY))", nil
	case r3.BucketMonth:
		return "DATE_FORMAT(" + col + ", '%Y-%m-01')", nil
	case r3.BucketYear:
		return "DATE_FORMAT(" + col + ", '%Y-01-01')", nil
	default:
		return "", fmt.Errorf("%w: unknown bucket unit %d", r3.ErrInvalidBucket, unit)
	}
}

// Pre-defined flavors for common databases.
var (
	FlavorPostgres = Flavor{
		Placeholder:          PlaceholderDollar,
		SupportsRETURNING:    true,
		TimestampFunc:        "NOW()",
		UsesOnConflictClause: true,
		BucketExpr:           postgresBucketExpr,
	}
	FlavorSQLite = Flavor{
		Placeholder:          PlaceholderQuestion,
		SupportsRETURNING:    true,
		TimestampFunc:        "datetime('now')",
		UsesOnConflictClause: true,
		BucketExpr:           sqliteBucketExpr,
	}
	FlavorMySQL = Flavor{
		Placeholder:       PlaceholderQuestion,
		SupportsRETURNING: false,
		TimestampFunc:     "CURRENT_TIMESTAMP",
		IdentifierQuote:   "`",
		BucketExpr:        mysqlBucketExpr,
	}
)

// Placeholders builds a placeholder list: "$1, $2, $3" from startIdx (dollar) or
// "?, ?, ?" (question).
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

// SetExprs builds an UPDATE SET body: "col1 = $1, col2 = $2, ..." or
// "col1 = ?, col2 = ?, ...".
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

// WhereEq builds "column = $N" or "column = ?". nextIdx is the 1-based parameter
// index (dollar-style only).
func (f Flavor) WhereEq(column string, nextIdx int) string {
	switch f.Placeholder {
	case PlaceholderQuestion:
		return column + " = ?"
	case PlaceholderDollar:
		return fmt.Sprintf("%s = $%d", column, nextIdx)
	}
	return column + " = ?" // fallback
}

// UpsertClause builds an upsert INSERT's trailing on-conflict clause:
// `ON CONFLICT (target) DO UPDATE SET col = EXCLUDED.col, ...` (Postgres/SQLite)
// or `ON DUPLICATE KEY UPDATE col = VALUES(col), ...` (MySQL). conflictCols is the
// target (MySQL ignores it, keying off any unique index). Empty updateCols yields
// DO NOTHING (a self-assign no-op on MySQL), leaving the existing row untouched.
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

	// MySQL: with no columns to update, self-assign the first conflict column so
	// the statement still parses and no-ops on duplicate.
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

// QuoteIdentifiers rewrites the dialect's ANSI double-quoted identifiers (`"col"`,
// `"tbl"."col"`) to this flavor's quote char; a no-op for ANSI flavors (Postgres,
// SQLite).
//
// Safe on a fully assembled query: values are always bound as parameters, never
// inlined, so the only double quotes in the SQL are identifier quotes. A doubled
// embedded quote (`""`, the dialect's escape) maps to the doubled target quote,
// which is also correct - so escaping survives. (Identifiers are validated to
// contain no quotes, so this never actually occurs.)
func (f Flavor) QuoteIdentifiers(sql string) string {
	if f.IdentifierQuote == "" || f.IdentifierQuote == `"` {
		return sql
	}
	return strings.ReplaceAll(sql, `"`, f.IdentifierQuote)
}

// ConvertPlaceholders rewrites `?` placeholders in clause to `$1, $2, ...` from
// startIdx, returning the result and the next free index. No-op (but still
// advances the index) for `?`-style flavors.
func (f Flavor) ConvertPlaceholders(clause string, startIdx int) (string, int) {
	if f.Placeholder == PlaceholderQuestion {
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
