package r3toml

import "github.com/amberpixels/r3"

// TOMLPagination represents pagination in TOML format.
type TOMLPagination struct {
	PageNum  int `toml:"page_num,omitempty"`
	PageSize int `toml:"page_size,omitempty"`
}

// ToPaginationSpec converts a TOMLPagination to an r3.PaginationSpec.
func (tp *TOMLPagination) ToPaginationSpec() (*r3.PaginationSpec, error) {
	if tp.PageNum <= 0 && tp.PageSize <= 0 {
		return r3.NoPagination(), nil
	}

	switch {
	case tp.PageNum > 0 && tp.PageSize > 0:
		return r3.NewPaginationSpec(tp.PageNum, tp.PageSize), nil
	case tp.PageNum > 0:
		return r3.NewPaginationSpec(tp.PageNum), nil
	default:
		return r3.NewPaginationSpecWithSize(tp.PageSize), nil
	}
}
