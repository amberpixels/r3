package r3sql

import (
	"fmt"

	"github.com/amberpixels/r3"
)

// JSONPathExprFunc renders the extraction of a key path from a JSON column.
// quotedCol is the already ANSI-quoted column; path segments are already
// validated identifiers. numeric selects a numeric comparison over text.
type JSONPathExprFunc func(quotedCol string, path []string, numeric bool) (string, error)

// RenderOptions carries the flavor-specific rendering this flavor-neutral
// dialect cannot do itself. The zero value renders plain fields only and rejects
// a JSON field with [r3.ErrJSONFieldNotSupported].
type RenderOptions struct {
	// JSONPathExpr renders a JSON field (see [r3.JSONField]); nil rejects one.
	JSONPathExpr JSONPathExprFunc
}

// SafeFieldExpr is [SafeColumnExpr] that also renders a JSON field through
// opts.JSONPathExpr after validating it. Filters, sorts, cursors and group-by
// render their fields through it.
func SafeFieldExpr(f *r3.FieldSpec, opts RenderOptions) (string, error) {
	if !f.IsJSONField() {
		return SafeColumnExpr(f)
	}
	name := f.String()
	if err := r3.ValidateJSONField(name); err != nil {
		return "", err
	}
	if opts.JSONPathExpr == nil {
		return "", fmt.Errorf("%w: %q", r3.ErrJSONFieldNotSupported, name)
	}
	col, path, numeric, _ := f.JSONPath()
	return opts.JSONPathExpr(QuoteIdentifier(col), path, numeric)
}

// jsonNumericArgs converts numeric-string args for a number-mode JSON field so
// a value parsed from a URL binds as a number; other fields pass through.
func jsonNumericArgs(f *r3.FieldSpec, args []any) []any {
	if _, _, numeric, ok := f.JSONPath(); !ok || !numeric {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = r3.JSONFieldNumericArg(a)
	}
	return out
}
