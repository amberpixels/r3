package depo

import "slices"

// ListParams defines parameters for filtering, sorting, and field selection.
type ListParams struct {
	Filters Filterables
	Sort    Sortables
	Fields  Fieldables
	Limit   int
	Offset  int

	IncludeTrashed bool         // Include trashed (soft-deleted) records.
	Preloads       Preloadables // List of related entities to preload.
}

// Clone returns a copy of the ListParams.
// It performs deep copying of slices where possible to avoid shared references.
func (lp ListParams) Clone() ListParams {
	// TODO fixme
	clone := lp

	if lp.Preloads != nil {
		clone.Preloads = slices.Clone(lp.Preloads)
	}

	// Clone Fields if it's a known concrete type.
	if lp.Fields != nil {
		switch v := lp.Fields.(type) {
		case StringFields:
			// StringFields is a slice of StringField (alias for string), so we copy it.
			clone.Fields = StringFields(append(StringFields(nil), v...))
		default:
			// Fallback: shallow copy of the interface value.
			clone.Fields = lp.Fields
		}
	}

	// Clone Filters if it's a known concrete type.
	if lp.Filters != nil {
		switch v := lp.Filters.(type) {
		case *FiltersGroup:
			// Create a new FiltersGroup and copy its filters slice.
			newFG := &FiltersGroup{filters: append(Filters(nil), v.filters...)}
			clone.Filters = newFG
		default:
			// Fallback: shallow copy.
			clone.Filters = lp.Filters
		}
	}

	// Clone Sort if it's a known concrete type.
	// Currently, we don't have a concrete sort type defined.
	// So we fall back to shallow copying. Extend this as needed.
	if lp.Sort != nil {
		clone.Sort = lp.Sort
	}

	// Primitive types (Limit, Offset, IncludeTrashed) are copied by value.
	return clone
}

// GetParams defines optional parameters for the Get operation.
type GetParams struct {
	Fields         Fieldables   // Specific fields to retrieve.
	IncludeTrashed bool         // Include trashed (soft-deleted) records.
	Preloads       Preloadables // List of related entities to preload.
}

// TODO fixme
func (gp GetParams) Clone() GetParams {
	return gp
}
