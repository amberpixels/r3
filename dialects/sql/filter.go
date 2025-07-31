package r3sql

import (
	"strings"

	"github.com/amberpixels/k1/quick"
)

// SQLClause represents a r3.Filter that can be converted into SQL Clause (used by gorm, sqlx, squirrel, etc.)
type SQLClause struct {
	// Clause is the raw safe SQL string (e.g. "name = ?")
	Clause string

	// Args is a slice of arguments to be bound to the SQL clause.
	// Len of arguments must match the number of placeholders in the clause.
	Args []any

	// Joins is a slice of joins that are required for the clause.
	Joins []SQLColumn
}

// SQLClauses is a slice of SQLClause.
type SQLClauses []SQLClause

// Joins is a combined getter for each SQLClause.Joins
// It will return duplicates-free list.
func (cs SQLClauses) Joins() []SQLColumn {
	var joins []SQLColumn
	for _, c := range cs {
		joins = quick.Append(joins, c.Joins...)
	}
	return joins
}

// extractJoinFromField inspects the field name and returns a slice with join information if necessary.
// For demonstration, assume join information is determined by a prefix separated by a dot.
// E.g., "user.name" could signal a join to the "user" table.
func extractJoinFromField(field SQLColumn) []SQLColumn {
	// If there is a dot, we assume the portion before the dot indicates a join.
	parts := strings.Split(string(field), ".")
	if len(parts) > 1 && parts[0] != "" {
		return []SQLColumn{SQLColumn(parts[0])}
	}
	return nil
}
