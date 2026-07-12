package r3sql

// SQLSort is a SQL sort clause, e.g. "name ASC", "created_at DESC NULLS LAST".
type SQLSort string

// String returns the SQL sort expression as a string.
func (ss SQLSort) String() string { return string(ss) }
