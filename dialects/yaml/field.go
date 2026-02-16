package r3yaml

// YAMLField is a string value representing a field name in YAML.
type YAMLField string

// YAMLFields is a collection of YAMLField values.
type YAMLFields []YAMLField

// String returns the string representation of the field.
func (f YAMLField) String() string { return string(f) }
