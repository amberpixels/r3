package r3

import "github.com/amberpixels/k1/maybe"

// ListParams defines parameters for filtering, sorting, and field selection.
type ListParams struct {
	GetParams

	Filters    Filters    `json:"filters"`
	Sorts      Sorts      `json:"sorts"`
	Pagination Pagination `json:"pagination"`

	// TODO(future): omit counting if specified (optimization)
}

func NewListParams() ListParams {
	return ListParams{
		GetParams: NewGetParams(),
	}
}

func DefaultListParams() ListParams {
	lp := NewListParams()
	lp.Pagination = DefaultPagination()

	return lp
}

// Clone returns a copy of the ListParams.
// It performs deep copying of slices where possible to avoid shared references.
func (lp ListParams) Clone() ListParams {
	clone := lp // TODO(issue#11): real deep clone
	return clone
}

// MergeWith merges given ListParams with some other ListParams.
func (lp ListParams) MergeWith(other ListParams) ListParams {
	result := lp.Clone()

	result.GetParams = result.GetParams.MergeWith(other.GetParams)

	result.Filters = result.Filters.MergeWith(other.Filters)
	result.Sorts = result.Sorts.MergeWith(other.Sorts)
	result.Pagination = result.Pagination.MergeWith(other.Pagination)

	return result
}

// GetParams defines optional parameters for the Get operation.
type GetParams struct {
	Fields   Fields   `json:"fields"`   // Specific fields to retrieve.
	Preloads Preloads `json:"preloads"` // List of related entities to preload.

	IncludeTrashed maybe.Bool `json:"include_trashed,omitzero"` // Include trashed (soft-deleted) records.
}

// NewGetParams returns empty GetParams
func NewGetParams() GetParams { return GetParams{} }

// DefaultGetParams returns detault set of GetParams. (Same as New for now)
func DefaultGetParams() GetParams { return NewGetParams() }

// MergeWith merges given GetParams with some other GetParams.
func (gp GetParams) MergeWith(other GetParams) GetParams {
	result := gp.Clone()

	result.Fields = result.Fields.MergeWith(other.Fields)
	result.Preloads = result.Preloads.MergeWith(other.Preloads)

	if other.IncludeTrashed.Some() {
		result.IncludeTrashed = other.IncludeTrashed
	}

	return result
}

// Clone clones GetParams.
func (gp GetParams) Clone() GetParams {
	var clone = gp // TODO(issue#11): real deep clone
	return clone
}
