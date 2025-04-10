package r3atoms

import "github.com/amberpixels/r3/internal/option"

const (
	// NoLimit is a special value for pagination limit that means no limit.
	// Here it's -1 but we'll check any negative value for infinite limit.
	NoLimit = -1
)

// Pagination represents the pagination parameters for a Repo request.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewPagination returns a new Pagination with the given limit and offset.
func NewPagination(limit, offset int) option.Option[Pagination] {
	return option.Some(Pagination{
		Limit: limit, Offset: offset,
	})
}

// NoLimitPagination returns a new Pagination with no limit and offset
func NoLimitPagination() option.Option[Pagination] {
	return option.Some(Pagination{Limit: NoLimit})
}

// TODO(better-repos@v2): gRPC controllers use PageNumber & PageSize for pagination
//       We'll need a way to convert (page, size) to (limit, offset).

// IsPaginated returns true if the pagination parameters are set.
func (p Pagination) IsPaginated() bool {
	return p.Limit >= 0 || p.Offset > 0
}

func (p Pagination) GetLimit(defaultLimit int) int {
	if p.Limit >= 0 {
		return p.Limit
	}
	return defaultLimit
}
