package r3

import "github.com/amberpixels/k1/maybe"

// GetParams defines parameters for the Get operation.
// For Get all params are considered to be optional.
type GetParams struct {
	Fields   Fields   // Specific fields to retrieve.
	Preloads Preloads // List of related entities to preload.

	// IncludeTrashed when true will still return trashed (soft-deleted) records.
	IncludeTrashed maybe.Bool
}

// NewGetParams returns empty GetParams.
func NewGetParams() GetParams { return GetParams{} }

// DefaultGetParams returns detault set of GetParams. (Same as New for now).
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
	var clone GetParams
	clone.Fields = gp.Fields.Clone()
	clone.Preloads = gp.Preloads.Clone()
	clone.IncludeTrashed = gp.IncludeTrashed
	return clone
}

// ListParams defines parameters for filtering, sorting, and field selection.
type ListParams struct {
	// GetParams are supported for List as well
	GetParams

	Filters    Filters    `json:"filters"`
	Sorts      Sorts      `json:"sorts"`
	Pagination Pagination `json:"pagination"`

	// TODO(future): omit counting if specified (optimization)
}

// NewListParams returns an empty (but ready-to-be-used) List Params.
func NewListParams() ListParams {
	return ListParams{GetParams: NewGetParams(), Pagination: NoLimitPagination()}
}

func DefaultListParams() ListParams {
	lp := NewListParams()
	lp.Pagination = DefaultPagination()

	return lp
}

// Clone returns a copy of the ListParams.
// It performs deep copying of slices where possible to avoid shared references.
func (lp ListParams) Clone() ListParams {
	var clone ListParams
	clone.GetParams = lp.GetParams.Clone()
	clone.Filters = lp.Filters.Clone()
	clone.Sorts = lp.Sorts.Clone()
	clone.Pagination = lp.Pagination.Clone()

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
