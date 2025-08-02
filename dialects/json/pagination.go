package r3json

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
)

// JSONPagination represents pagination in JSON format using page_num and page_size.
type JSONPagination struct {
	PageNum  int `json:"page_num,omitempty"`
	PageSize int `json:"page_size,omitempty"`
}

// String returns the JSON string representation of pagination.
func (jp *JSONPagination) String() string {
	bytes, err := json.Marshal(jp)
	if err != nil {
		return fmt.Sprintf("<invalid pagination: %s>", err.Error())
	}
	return string(bytes)
}

// ToPaginationSpec converts JSONPagination to r3.PaginationSpec.
func (jp *JSONPagination) ToPaginationSpec() (*r3.PaginationSpec, error) {
	if jp.PageNum <= 0 && jp.PageSize <= 0 {
		return r3.NoPagination(), nil
	}

	var pagination *r3.PaginationSpec
	switch {
	case jp.PageNum > 0 && jp.PageSize > 0:
		pagination = r3.NewPaginationSpec(jp.PageNum, jp.PageSize)
	case jp.PageNum > 0:
		pagination = r3.NewPaginationSpec(jp.PageNum)
	default:
		pagination = r3.NewPaginationSpecWithSize(jp.PageSize)
	}

	return pagination, nil
}

// Helper functions for dialect.go

// jsonPaginationFromR3 converts r3.PaginationSpec to JSONPagination.
func jsonPaginationFromR3(p *r3.PaginationSpec) *JSONPagination {
	if p == nil {
		return &JSONPagination{}
	}

	return &JSONPagination{
		PageNum:  p.GetPageNum(),
		PageSize: p.GetPageSize(),
	}
}
