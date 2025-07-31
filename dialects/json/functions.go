package r3json

import (
	"github.com/amberpixels/r3"
)

// func FiltersToSQLClauses(filters r3.Filters) (SQLClauses, error) {

func JSONFiltersToFilters(inboundFilters JSONFilters) (r3.Filters, error) {
	return inboundFilters.toFilters()
}
