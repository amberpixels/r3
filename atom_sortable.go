package crood

import (
	"fmt"
	"strings"
)

type Order int

const (
	ASC Order = iota
	DESC
)

func (o Order) String() string {
	switch o {
	case ASC:
		return "asc"
	case DESC:
		return "desc"
	default:
		panic("unknown sort order")
	}
}

// SortCriteria defines a single sorting rule with a field and order (e.g., ASC or DESC).
type Sortable interface {
	GetField() Fieldable // Field to sort by, e.g., "created_at"
	GetOrder() Order     // Sorting order, e.g., "asc" or "desc"
}

type Sortables interface {
	GetSortCriterias() SortCriterias
}

// SortCriteria defines a single sorting rule with a field and order (e.g., ASC or DESC).
type SortCriteria struct {
	Field Fieldable // Field to sort by, e.g., "created_at"
	Order Order     // Sorting order, e.g., "asc" or "desc"
}

func (s SortCriteria) String() string {
	return fmt.Sprintf("%s %s", s.Field, s.Order)
}

// SortCriterias is a slice of SortCriteria.
type SortCriterias []SortCriteria

func (s SortCriterias) String() string {
	var strs []string
	for _, sc := range s {
		strs = append(strs, sc.String())
	}
	return strings.Join(strs, ",")
}

func (s SortCriterias) GetSortCriterias() SortCriterias {
	return s
}
