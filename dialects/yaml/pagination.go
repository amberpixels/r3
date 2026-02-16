package r3yaml

import "github.com/amberpixels/r3"

// YAMLPagination represents pagination in YAML format.
type YAMLPagination struct {
	PageNum  int `yaml:"page_num,omitempty"`
	PageSize int `yaml:"page_size,omitempty"`
}

// ToPaginationSpec converts a YAMLPagination to an r3.PaginationSpec.
func (yp *YAMLPagination) ToPaginationSpec() (*r3.PaginationSpec, error) {
	if yp.PageNum <= 0 && yp.PageSize <= 0 {
		return r3.NoPagination(), nil
	}

	switch {
	case yp.PageNum > 0 && yp.PageSize > 0:
		return r3.NewPaginationSpec(yp.PageNum, yp.PageSize), nil
	case yp.PageNum > 0:
		return r3.NewPaginationSpec(yp.PageNum), nil
	default:
		return r3.NewPaginationSpecWithSize(yp.PageSize), nil
	}
}
