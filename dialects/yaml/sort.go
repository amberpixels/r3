package r3yaml

import (
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// YAMLSort represents a sort criteria in YAML format.
type YAMLSort struct {
	Field         string `yaml:"field"`
	Direction     string `yaml:"direction,omitempty"`
	NullsPosition string `yaml:"nulls_position,omitempty"`
}

// ToSortSpec converts a YAMLSort to an r3.SortSpec.
func (ys *YAMLSort) ToSortSpec() (*r3.SortSpec, error) {
	if ys == nil {
		return nil, newError(fmt.Errorf("nil YAML sort"))
	}

	if ys.Field == "" {
		return nil, newError(fmt.Errorf("empty field in YAML sort"))
	}

	return &r3.SortSpec{
		Column:        r3.NewFieldSpec(ys.Field),
		Direction:     canonical.ParseSortDirection(ys.Direction),
		NullsPosition: canonical.ParseNullsPosition(ys.NullsPosition),
	}, nil
}
