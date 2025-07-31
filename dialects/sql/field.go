package r3sql

// SQLColumn stands for the string type that holds name of the column.
type SQLColumn string

func (sc SQLColumn) String() string { return string(sc) }
