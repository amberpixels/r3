package r3json

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

type Field string

type Fields []Field

func (f Field) String() string { return string(f) }

func (fields Fields) String() string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.String()
	}
	return strings.Join(parts, ",")
}

var _ r3.FilterInboundDialector = (*JSONInboundDialector)(nil)

func (d *JSONInboundDialector) ToField(dialectValue r3.DialectValue) (r3.Field, error) {
	inboundFilter, ok := dialectValue.(Field)
	if !ok {
		if ptr, ok := dialectValue.(*Field); ok {
			inboundFilter = *ptr
		} else {
			return nil, fmt.Errorf("invalid filter type: %T", dialectValue)
		}
	}

	return inboundFilter.ToColumnField()
}

func (d *JSONInboundDialector) ToFields(dialectValue r3.DialectValue) (r3.Fields, error) {
	inboundFields, ok := dialectValue.(Fields)
	if !ok {
		inboundFilter, ok := dialectValue.(Field)
		if !ok {
			if ptr, ok := dialectValue.(*Field); ok {
				inboundFilter = *ptr
			} else {
				return nil, fmt.Errorf("invalid field type: %T", dialectValue)
			}
		}

		inboundFields = Fields{inboundFilter}
	}

	return inboundFields.ToColumnFields()
}

func JSONFieldsToFields(fields Fields) (r3.Fields, error) {
	return (&JSONInboundDialector{}).ToFields(fields)
}

// UnmarshalJSON is optional, depending on how you want to handle the Value.
func (f *Field) UnmarshalJSON(data []byte) error {
	type alias Field
	return json.Unmarshal(data, (*alias)(f))
}

func (fields Fields) ToColumnFields() (r3.Fields, error) {
	columnFields := make(r3.Fields, len(fields))
	for i, f := range fields {
		columnField, err := f.ToColumnField()
		if err != nil {
			return nil, err
		}
		columnFields[i] = columnField
	}
	return columnFields, nil
}

func (f Field) ToColumnField() (r3.ColumnField, error) {
	return r3.ColumnField(f), nil
}
