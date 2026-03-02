package enginefile

import (
	"github.com/amberpixels/r3"
)

// filterCursorPosition filters the sorted entity slice to include only items
// after (or before) the cursor position, based on the sort columns and direction.
//
// This is the file-engine equivalent of keyset pagination: after sorting, we
// skip items that are "before" the cursor position according to the sort order.
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

	// Find the first entity that should be included (i.e., past the cursor).
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

// entityPastCursor returns true if the entity is past the cursor position
// (should be included in results) given the sort columns and direction.
func entityPastCursor[T any](
	entity T,
	values r3.CursorValues,
	sorts r3.Sorts,
	direction r3.CursorDirection,
	meta *StructMeta,
) bool {
	// For each sort column level, check if the entity value moves past the cursor.
	// This mirrors the SQL compound WHERE logic:
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
			// Not the last sort column
			if cmp == 0 {
				// Equal on this column, check next column
				continue
			}
			// Not equal: check if it's in the right direction
			return isCorrectDirection(cmp, sort.Direction, direction)
		}

		// Last sort column: must be strictly past (not equal)
		if cmp == 0 {
			return false
		}
		return isCorrectDirection(cmp, sort.Direction, direction)
	}

	return false
}

// isCorrectDirection returns true if the comparison result indicates the entity
// is past the cursor in the expected direction.
func isCorrectDirection(cmp int, sortDir r3.SortDirection, cursorDir r3.CursorDirection) bool {
	// For forward + DESC: entity should have smaller values (cmp < 0)
	// For forward + ASC: entity should have larger values (cmp > 0)
	// Backward flips this.
	wantSmaller := sortDir != r3.SortDirectionAsc // DESC or Unspecified -> want smaller
	if cursorDir == r3.CursorBackward {
		wantSmaller = !wantSmaller
	}

	if wantSmaller {
		return cmp < 0
	}
	return cmp > 0
}
