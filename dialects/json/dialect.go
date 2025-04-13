package r3json

import "github.com/amberpixels/r3"

type JSONInboundDialector struct{}

var _ r3.FieldInboundDialector = (*JSONInboundDialector)(nil)
