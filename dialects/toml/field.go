package r3toml

// TOMLField is a string value representing a field name in TOML.
type TOMLField string

// TOMLFields is a collection of TOMLField values.
type TOMLFields []TOMLField

// String returns the string representation of the field.
func (f TOMLField) String() string { return string(f) }
