package r3json

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
)

// JSONInboundDialector converts JSON dialect values to r3 types.
type JSONInboundDialector struct{}

// JSONOutboundDialector converts r3 types to JSON dialect values.
type JSONOutboundDialector struct{}

var (
	_ r3.FieldInboundDialector          = (*JSONInboundDialector)(nil)
	_ r3.FilterInboundDialector         = (*JSONInboundDialector)(nil)
	_ r3.FilterOperatorInboundDialector = (*JSONInboundDialector)(nil)

	_ r3.FieldOutboundDialector          = (*JSONOutboundDialector)(nil)
	_ r3.FilterOutboundDialector         = (*JSONOutboundDialector)(nil)
	_ r3.FilterOperatorOutboundDialector = (*JSONOutboundDialector)(nil)
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

// JSONOutboundDialector methods - convert r3 types to JSON dialect values

// TranslateFieldSpec converts a FieldSpec to a JSON field.
func (d *JSONOutboundDialector) TranslateFieldSpec(field *r3.FieldSpec) (r3.DialectValue, error) {
	if field == nil {
		return nil, newError(errors.New("nil field spec"))
	}

	jsonField := JSONField(field.String())

	return jsonField, nil
}

// TranslateFilterSpec converts a FilterSpec to a JSON filter.
func (d *JSONOutboundDialector) TranslateFilterSpec(filter *r3.FilterSpec) (r3.DialectValue, error) {
	if filter == nil {
		return nil, newError(errors.New("nil filter spec"))
	}

	jsonFilter := &JSONFilter{}

	// Handle simple field-operator-value filters
	if filter.Field != nil {
		jsonFilter.Field = JSONField(filter.Field.String())
		jsonFilter.Value = filter.Value

		// Convert operator
		jsonOp, err := r3ToJSONOperator(filter.Operator)
		if err != nil {
			return nil, newError(fmt.Errorf("failed to convert operator: %w", err))
		}
		jsonFilter.Op = jsonOp
	}

	// Note: AND/OR group handling would require extending the JSON schema
	// For now, we only handle simple field-operator-value filters
	if len(filter.And) > 0 || len(filter.Or) > 0 {
		return nil, newError(errors.New("AND/OR group conversion not yet implemented"))
	}

	return jsonFilter, nil
}

// TranslateFilterOperatorSpec converts a FilterOperatorSpec to a JSON operator.
func (d *JSONOutboundDialector) TranslateFilterOperatorSpec(op *r3.FilterOperatorSpec) (r3.DialectValue, error) {
	if op == nil {
		return nil, newError(errors.New("nil filter operator spec"))
	}

	jsonOp, err := r3ToJSONOperator(*op)
	if err != nil {
		return nil, newError(fmt.Errorf("failed to convert r3 operator to JSON: %w", err))
	}

	return jsonOp, nil
}

// Helper function to convert r3 operators to JSON operators.
func r3ToJSONOperator(op r3.FilterOperatorSpec) (JSONFilterOperator, error) {
	switch op {
	case r3.OperatorEq:
		return OperatorEq, nil
	case r3.OperatorNe:
		return OperatorNe, nil
	case r3.OperatorExists:
		return OperatorExists, nil
	case r3.OperatorGt:
		return OperatorGt, nil
	case r3.OperatorGte:
		return OperatorGte, nil
	case r3.OperatorLt:
		return OperatorLt, nil
	case r3.OperatorLte:
		return OperatorLte, nil
	case r3.OperatorBetween:
		return OperatorBetween, nil
	case r3.OperatorBetweenEx:
		return OperatorBetweenEx, nil
	case r3.OperatorBetweenExInc:
		return OperatorBetweenExInc, nil
	case r3.OperatorBetweenIncEx:
		return OperatorBetweenIncEx, nil
	case r3.OperatorIn:
		return OperatorIn, nil
	case r3.OperatorNotIn:
		return OperatorNotIn, nil
	case r3.OperatorLike:
		return OperatorLike, nil
	case r3.OperatorNotLike:
		return OperatorNotLike, nil
	case r3.OperatorILike:
		return OperatorILike, nil
	case r3.OperatorUnspecified:
		fallthrough
	default:
		return OperatorUnspecified, fmt.Errorf("unsupported r3 filter operator: %v", op)
	}
}
