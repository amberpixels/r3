package r3sql

import (
	"github.com/amberpixels/r3"
)

type ColumnField string

func (sv *SqlDialector) FromColumnField(cf r3.ColumnField) (r3.DialectValue, error) {
	return ColumnField(cf.String()), nil
}
