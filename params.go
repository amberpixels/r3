package crood

// PreloadList is a slice of Preload.
type PreloadList []Preload

// Preload defines a single preload rule.
type Preload interface {
	GetName() string                // The name of the related entity, e.g., "Author".
	GetNestedPreloads() PreloadList // Nested preloads, e.g., "Author.Books".
}

// ListParams defines parameters for filtering, sorting, and field selection.
type ListParams struct {
	Filters FilterList
	Sort    SortCriteriaList
	Fields  FieldList
	Limit   int
	Offset  int

	IncludeTrashed bool        // Include trashed (soft-deleted) records.
	Preloads       PreloadList // List of related entities to preload.
}

// GetParams defines optional parameters for the Get operation.
type GetParams struct {
	Fields         FieldList   // Specific fields to retrieve.
	IncludeTrashed bool        // Include trashed (soft-deleted) records.
	Preloads       PreloadList // List of related entities to preload.
}

// UpdateParams defines optional parameters for the Update operation.
type UpdateParams struct {
	Fields FieldList // Specific fields to update.

	// TODO We should support conditional updates e.g. Filters support
}
