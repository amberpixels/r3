package r3atoms

import "github.com/amberpixels/r3/internal/option"

// ListParams defines parameters for filtering, sorting, and field selection.
type ListParams struct {
	GetParams

	Filters    Filters                   `json:"filters"`
	Sorts      Sorts                     `json:"sorts"`
	Pagination option.Option[Pagination] `json:"pagination"`
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
	Fields   Fields   `json:"fields"`   // Specific fields to retrieve.
	Preloads Preloads `json:"preloads"` // List of related entities to preload.

	IncludeTrashed option.Bool `json:"include_trashed,omitzero"` // Include trashed (soft-deleted) records.
}

// Clone clones GetParams
func (gp GetParams) Clone() GetParams {
	// TODO(p2): real deep clone
	return gp
}
