package canonical

import (
	"fmt"

	"github.com/amberpixels/r3"
)

// aggregateByName fixes the names now, even before json/yaml/toml/url wire up
// aggregate serialization, so those dialects converge on them later.
var aggregateByName = map[string]r3.AggregateFunc{
	"count":          r3.AggregateCount,
	"count_distinct": r3.AggregateCountDistinct,
	"sum":            r3.AggregateSum,
	"avg":            r3.AggregateAvg,
	"min":            r3.AggregateMin,
	"max":            r3.AggregateMax,
}

// nameByAggregate is the reverse of aggregateByName.
var nameByAggregate = func() map[r3.AggregateFunc]string {
	m := make(map[r3.AggregateFunc]string, len(aggregateByName))
	for name, fn := range aggregateByName {
		if _, exists := m[fn]; !exists {
			m[fn] = name
		}
	}
	return m
}()

// ParseAggregateFunc parses a canonical aggregate-function string ("count",
// "count_distinct", "sum", "avg", "min", "max") to r3.AggregateFunc.
func ParseAggregateFunc(s string) (r3.AggregateFunc, error) {
	fn, ok := aggregateByName[s]
	if !ok {
		return 0, fmt.Errorf("unknown canonical aggregate function: %q", s)
	}
	return fn, nil
}

// FormatAggregateFunc returns the canonical string for an r3.AggregateFunc.
func FormatAggregateFunc(fn r3.AggregateFunc) string {
	if name, ok := nameByAggregate[fn]; ok {
		return name
	}
	return ""
}
