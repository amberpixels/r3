package r3sql

// SQLSort stands for the string type that holds a SQL sort clause.
// Example: "name ASC", "created_at DESC NULLS LAST".
type SQLSort string

func (ss SQLSort) String() string { return string(ss) }
