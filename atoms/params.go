package r3atoms

// ListParams defines parameters for filtering, sorting, and field selection.
type ListParams struct {
	GetParams

	Filters    Filters    `json:"filters"`
	Sorts      Sorts      `json:"sorts"`
	Pagination Pagination `json:"pagination"`
}

// Clone returns a copy of the ListParams.
// It performs deep copying of slices where possible to avoid shared references.
func (lp ListParams) Clone() ListParams {
	// TODO(p2): real deep clone
	clone := lp
	return clone
}

// GetParams defines optional parameters for the Get operation.
type GetParams struct {
	Fields         Fields   `json:"fields"`          // Specific fields to retrieve.
	IncludeTrashed bool     `json:"include_trashed"` // Include trashed (soft-deleted) records.
	Preloads       Preloads `json:"preloads"`        // List of related entities to preload.
}

// Clone clones GetParams
func (gp GetParams) Clone() GetParams {
	// TODO(p2): real deep clone
	return gp
}
