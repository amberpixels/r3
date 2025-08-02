package r3sql

import "fmt"

// SQLPagination represents pagination in SQL format using LIMIT and OFFSET.
type SQLPagination struct {
	Limit  int
	Offset int
}

// String returns the string representation of SQL pagination.
func (sp *SQLPagination) String() string {
	if sp.Limit > 0 && sp.Offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", sp.Limit, sp.Offset)
	}
	if sp.Limit > 0 {
		return fmt.Sprintf("LIMIT %d", sp.Limit)
	}
	if sp.Offset > 0 {
		return fmt.Sprintf("OFFSET %d", sp.Offset)
	}
	return ""
}

// ToLimitOffset returns the limit and offset values.
func (sp *SQLPagination) ToLimitOffset() (int, int) {
	return sp.Limit, sp.Offset
}

// NewSQLPagination creates a new SQLPagination with given limit and offset.
func NewSQLPagination(limit, offset int) *SQLPagination {
	return &SQLPagination{
		Limit:  limit,
		Offset: offset,
	}
}
