package enginefile

import (
	"github.com/amberpixels/r3"
)

// filterCursorPosition keeps only the entities past the cursor for the sort
// columns and direction - the file-engine equivalent of SQL keyset pagination,
// applied after sorting.
func filterCursorPosition[T any](
	entities []T,
	values r3.CursorValues,
	sorts r3.Sorts,
	direction r3.CursorDirection,
	meta *StructMeta,
) []T {
	if len(values) == 0 || len(entities) == 0 {
		return entities
	}

	// First entity past the cursor.
	startIdx := -1
	for i, entity := range entities {
		if entityPastCursor(entity, values, sorts, direction, meta) {
			startIdx = i
			break
		}
	}

	if startIdx < 0 {
		return nil
	}
	return entities[startIdx:]
}

// entityPastCursor reports whether the entity sorts past the cursor position for
// the given columns and direction.
func entityPastCursor[T any](
	entity T,
	values r3.CursorValues,
	sorts r3.Sorts,
	direction r3.CursorDirection,
	meta *StructMeta,
) bool {
	// Mirrors the SQL compound keyset WHERE:
	//   (col1 < ?) OR (col1 = ? AND col2 > ?) OR ...
	for i, sort := range sorts {
		col := sort.Column.String()
		entityVal, ok := meta.GetFieldValue(entity, col)
		if !ok {
			continue
		}

		cursorVal := values[col]
		cmp := compareOrdered(entityVal, cursorVal)

		if i < len(sorts)-1 {
			if cmp == 0 {
				continue // tie on this column: fall through to the next
			}
			return isCorrectDirection(cmp, sort.Direction, direction)
		}

		// Last column: a tie is not strictly past the cursor.
		if cmp == 0 {
			return false
		}
		return isCorrectDirection(cmp, sort.Direction, direction)
	}

	return false
}

// isCorrectDirection reports whether cmp puts the entity past the cursor.
// Forward wants smaller values under DESC and larger under ASC; backward flips it.
func isCorrectDirection(cmp int, sortDir r3.SortDirection, cursorDir r3.CursorDirection) bool {
	wantSmaller := sortDir != r3.SortDirectionAsc // DESC or Unspecified -> want smaller
	if cursorDir == r3.CursorBackward {
		wantSmaller = !wantSmaller
	}

	if wantSmaller {
		return cmp < 0
	}
	return cmp > 0
}
