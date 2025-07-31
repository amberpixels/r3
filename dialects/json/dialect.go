package r3json

import (
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/internal/notimplemented"
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

// TranslateIntoFilterOperatorSpec converts a JSON-dialect filter operator into a JSONFilterOperator.
func (d *JSONInboundDialector) TranslateIntoFilterOperatorSpec(op r3.DialectValue) (*r3.FilterOperatorSpec, error) {
	// TODO: implement me!
	_ = op
	return nil, notimplemented.Err
}
