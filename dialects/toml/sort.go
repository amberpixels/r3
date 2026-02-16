package r3toml

import (
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// TOMLSort represents a sort criteria in TOML format.
type TOMLSort struct {
	Field         string `toml:"field"`
	Direction     string `toml:"direction,omitempty"`
	NullsPosition string `toml:"nulls_position,omitempty"`
}

// ToSortSpec converts a TOMLSort to an r3.SortSpec.
func (ts *TOMLSort) ToSortSpec() (*r3.SortSpec, error) {
	if ts == nil {
		return nil, newError(fmt.Errorf("nil TOML sort"))
	}

	if ts.Field == "" {
		return nil, newError(fmt.Errorf("empty field in TOML sort"))
	}

	return &r3.SortSpec{
		Column:        r3.NewFieldSpec(ts.Field),
		Direction:     canonical.ParseSortDirection(ts.Direction),
		NullsPosition: canonical.ParseNullsPosition(ts.NullsPosition),
	}, nil
}
