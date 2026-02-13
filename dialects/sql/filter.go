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
	if joins == nil {
		return []SQLColumn{}
	}
	return joins
}

// extractJoinFromField inspects the field name and returns a slice with join information if necessary.
// The field name is the raw (unquoted) identifier path. If it contains a dot, the portion before
// the first dot indicates a join table. The table name is validated and quoted.
// E.g., "user.name" signals a join to the "user" table and returns ['"user"'].
func extractJoinFromField(fieldName string) ([]SQLColumn, error) {
	parts := strings.Split(fieldName, ".")
	if len(parts) > 1 && parts[0] != "" {
		quoted, err := safeJoinTable(parts[0])
		if err != nil {
			return nil, err
		}
		return []SQLColumn{quoted}, nil
	}
	return nil, nil
}
