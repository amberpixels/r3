package r3json

/*
func (d *JSONInboundDialector) ToFields(dialectValue r3.DialectValue) (r3.Fields, error) {
	inboundFields, ok := dialectValue.(JSONFields)
	if !ok {
		inboundFilter, ok := dialectValue.(JSONField)
		if !ok {
			if ptr, ok := dialectValue.(*JSONField); ok {
				inboundFilter = *ptr
			} else {
				return nil, fmt.Errorf("invalid field type: %T", dialectValue)
			}
		}

		inboundFields = JSONFields{inboundFilter}
	}

	return inboundFields.ToFieldSpecs()
}
*/

/*
func JSONFieldsToFields(inboundFields JSONFields) (r3.Fields, error) {
	return inboundFields.ToFieldSpecs()
}
*/
