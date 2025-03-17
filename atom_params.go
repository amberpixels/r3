package depo

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

// GetParams defines optional parameters for the Get operation.
type GetParams struct {
	Fields         Fieldables   // Specific fields to retrieve.
	IncludeTrashed bool         // Include trashed (soft-deleted) records.
	Preloads       Preloadables // List of related entities to preload.
}
