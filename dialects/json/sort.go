package r3json

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// JSONSort represents a sort criteria in JSON format.
type JSONSort struct {
	Field         JSONField         `json:"field"`
	Direction     JSONSortDirection `json:"direction,omitempty"`
	NullsPosition JSONNullsPosition `json:"nulls_position,omitempty"`
}

// JSONSortDirection represents sort direction in JSON.
type JSONSortDirection string

const (
	// JSONSortDirectionAsc represents ascending sort.
	JSONSortDirectionAsc JSONSortDirection = "asc"
	// JSONSortDirectionDesc represents descending sort.
	JSONSortDirectionDesc JSONSortDirection = "desc"
	// JSONSortDirectionUnspecified represents unspecified direction.
	JSONSortDirectionUnspecified JSONSortDirection = ""
)

// JSONNullsPosition represents nulls position in JSON.
type JSONNullsPosition string

const (
	// JSONNullsPositionFirst represents nulls first.
	JSONNullsPositionFirst JSONNullsPosition = "first"
	// JSONNullsPositionLast represents nulls last.
	JSONNullsPositionLast JSONNullsPosition = "last"
	// JSONNullsPositionUnspecified represents unspecified nulls position.
	JSONNullsPositionUnspecified JSONNullsPosition = ""
)

// String returns the string representation of the sort.
func (js *JSONSort) String() string {
	bytes, err := json.Marshal(js)
	if err != nil {
		return fmt.Sprintf("<invalid sort: %s>", err.Error())
	}
	return string(bytes)
}

// ToSortSpec converts JSONSort to r3.SortSpec.
func (js *JSONSort) ToSortSpec() (*r3.SortSpec, error) {
	fieldSpec, err := js.Field.ToFieldSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to convert field: %w", err)
	}

	return &r3.SortSpec{
		Column:        fieldSpec,
		Direction:     js.Direction.ToR3SortDirection(),
		NullsPosition: js.NullsPosition.ToR3NullsPosition(),
	}, nil
}

// ToR3SortDirection converts JSONSortDirection to r3.SortDirection.
func (jsd JSONSortDirection) ToR3SortDirection() r3.SortDirection {
	return canonical.ParseSortDirection(string(jsd))
}

// ToR3NullsPosition converts JSONNullsPosition to r3.SortNullsPosition.
func (jnp JSONNullsPosition) ToR3NullsPosition() r3.SortNullsPosition {
	return canonical.ParseNullsPosition(string(jnp))
}

// Helper functions for dialect.go

// jsonSortDirectionFromR3 converts r3.SortDirection to JSONSortDirection.
func jsonSortDirectionFromR3(direction r3.SortDirection) JSONSortDirection {
	return JSONSortDirection(canonical.FormatSortDirection(direction))
}

// jsonNullsPositionFromR3 converts r3.SortNullsPosition to JSONNullsPosition.
func jsonNullsPositionFromR3(position r3.SortNullsPosition) JSONNullsPosition {
	return JSONNullsPosition(canonical.FormatNullsPosition(position))
}
