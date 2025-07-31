package r3sql

import (
	"fmt"

	"github.com/amberpixels/r3"
)

// FiltersToSQLClauses translates list of r3.Filters into SQLClauses.
func FiltersToSQLClauses(filters r3.Filters) (SQLClauses, error) {
	if len(filters) == 0 {
		return nil, nil
	}

	d := &SQLDialector{}

	result := make(SQLClauses, len(filters))
	for i, f := range filters {
		clauseRaw, err := f.ToDialect(d)
		if err != nil {
			return nil, newError(err)
		}

		clause, ok := clauseRaw.(SQLClause)
		if !ok {
			return nil, newError(fmt.Errorf("unexpected clause type: %T", clauseRaw))
		}

		result[i] = clause
	}

	return result, nil
}
