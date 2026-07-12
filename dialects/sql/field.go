package r3sql

// SQLColumn is a column name.
type SQLColumn string

// String returns the column name as a string.
func (sc SQLColumn) String() string { return string(sc) }
