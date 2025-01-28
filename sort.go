package crood

// SortCriteria defines a single sorting rule with a field and order (e.g., ASC or DESC).
type SortCriteria interface {
	GetField() string // Field to sort by, e.g., "created_at"
	GetOrder() string // Sorting order, e.g., "asc" or "desc"
}

// SortCriteriaList is a slice of SortCriteria.
type SortCriteriaList []SortCriteria
