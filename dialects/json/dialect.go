package r3json

import "github.com/amberpixels/r3"

type JsonInboundDialector struct{}

var _ r3.FieldInboundDialector = (*JsonInboundDialector)(nil)
