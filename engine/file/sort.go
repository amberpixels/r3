package enginefile

import (
	"sort"

	"github.com/amberpixels/r3"
)

// sortEntities sorts entities in-place according to the given sort specifications.
func sortEntities[T any](entities []T, sorts r3.Sorts, meta *StructMeta) {
	if len(sorts) == 0 {
		return
	}

	sort.SliceStable(entities, func(i, j int) bool {
		for _, spec := range sorts {
			if spec == nil || spec.Column == nil {
				continue
			}

			fieldName := spec.Column.String()
			aVal, aOk := meta.GetFieldValue(entities[i], fieldName)
			bVal, bOk := meta.GetFieldValue(entities[j], fieldName)

			if !aOk && !bOk {
				continue
			}

			aIsNil := !aOk || isNullValue(aVal)
			bIsNil := !bOk || isNullValue(bVal)

			if aIsNil || bIsNil {
				if aIsNil && bIsNil {
					continue
				}
				return handleNullsPosition(aIsNil, bIsNil, spec.NullsPosition, spec.Direction)
			}

			cmp := compareOrdered(aVal, bVal)
			if cmp == 0 {
				continue
			}

			if spec.Direction == r3.SortDirectionDesc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

// handleNullsPosition determines ordering when one value is nil.
func handleNullsPosition(aIsNil, bIsNil bool, nullsPos r3.SortNullsPosition, dir r3.SortDirection) bool {
	if aIsNil && !bIsNil {
		switch nullsPos {
		case r3.NullsPositionFirst:
			return true
		case r3.NullsPositionLast:
			return false
		case r3.NullsPositionNotSpecified:
			// Default: nulls last for ASC, nulls first for DESC.
			return dir == r3.SortDirectionDesc
		}
	}
	// bIsNil && !aIsNil
	switch nullsPos {
	case r3.NullsPositionFirst:
		return false
	case r3.NullsPositionLast:
		return true
	case r3.NullsPositionNotSpecified:
		return dir != r3.SortDirectionDesc
	}

	return false
}
