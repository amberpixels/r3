package r3json

import (
	"fmt"

	"github.com/amberpixels/r3"
)

type JSONInboundDialector struct{}

var (
	_ r3.FieldInboundDialector          = (*JSONInboundDialector)(nil)
	_ r3.FilterInboundDialector         = (*JSONInboundDialector)(nil)
	_ r3.FilterOperatorInboundDialector = (*JSONInboundDialector)(nil)
)

// TranslateIntoFieldSpec converts a JSON-dialect field into a FieldSpec.
func (d *JSONInboundDialector) TranslateIntoFieldSpec(jsonField r3.DialectValue) (*r3.FieldSpec, error) {
	inboundFilter, ok := jsonField.(JSONField)
	if !ok {
		if ptr, ok := jsonField.(*JSONField); ok {
			inboundFilter = *ptr
		} else {
			return nil, newError(fmt.Errorf("invalid filter type: %T", jsonField))
		}
	}

	return inboundFilter.ToFieldSpec()
}

// TranslateIntoFilterSpec converts a JSON-dialect filter into a FilterSpec.
func (d *JSONInboundDialector) TranslateIntoFilterSpec(dialectValue r3.DialectValue) (*r3.FilterSpec, error) {
	inboundFilter, ok := dialectValue.(*JSONFilter)
	if !ok {
		return nil, newError(fmt.Errorf("invalid filter type: %T", dialectValue))
	}

	return inboundFilter.ToFilterSpec()
}

// TranslateIntoFilterOperatorSpec converts a JSON-dialect filter operator into a FilterOperatorSpec.
func (d *JSONInboundDialector) TranslateIntoFilterOperatorSpec(op r3.DialectValue) (*r3.FilterOperatorSpec, error) {
	jsonOp, ok := op.(JSONFilterOperator)
	if !ok {
		if ptr, ok := op.(*JSONFilterOperator); ok {
			jsonOp = *ptr
		} else if str, ok := op.(string); ok {
			// Handle string input by parsing it
			if err := jsonOp.UnmarshalText([]byte(str)); err != nil {
				return nil, newError(fmt.Errorf("invalid filter operator string: %s", str))
			}
		} else {
			return nil, newError(fmt.Errorf("invalid filter operator type: %T", op))
		}
	}

	r3Op, err := jsonOp.ToFilterOperatorSpec()
	if err != nil {
		return nil, newError(fmt.Errorf("failed to convert JSON operator to r3 operator: %w", err))
	}

	return &r3Op, nil
}
