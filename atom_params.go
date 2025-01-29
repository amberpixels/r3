package crood

// Preload defines a single preload rule.
type Preloadable interface {
	GetName() string                // The name of the related entity, e.g., "Author".
	GetNestedPreloads() Preloadable // Nested preloads, e.g., "Author.Books".
}

// Preloadables is a slice of Preloadable.
type Preloadables []Preloadable

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
