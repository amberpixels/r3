package crood

// Field defines a single field selection rule.
type Field interface {
	GetName() string         // Field name, e.g., "id" or "name"
	GetSubfields() FieldList // Nested fields, e.g., "address.city"
}

// FieldList is a slice of Field.
type FieldList []Field

func (fl FieldList) GetNames() []string {
	names := make([]string, len(fl))
	for i, f := range fl {
		names[i] = f.GetName()
	}
	return names
}
