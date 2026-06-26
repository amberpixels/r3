package enginesql

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/amberpixels/r3"
)

// sqlTimeType is time.Time, used to recognize managed-timestamp fields.
var sqlTimeType = reflect.TypeFor[time.Time]()

// WriteColumns returns the subset of cols that the given write op may write
// under the schema. The capability check is skipped — all cols are returned —
// when the write guard is bypassed (audited system/worker write) or no schema
// is present (back-compat for schema-less callers). The structural floor is
// enforced separately by the caller (the PK is never in cols; computed
// attributes have no column at all), so it can never be widened here.
func WriteColumns(ctx context.Context, schema r3.Schema, cols []string, op r3.WriteOp) []string {
	if schema.IsZero() || r3.WriteGuardBypassed(ctx) {
		return cols
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if schema.Writable(c, op) {
			out = append(out, c)
		}
	}
	return out
}

// RequireMutableColumns enforces the schema's Mutable capability for an explicit
// Patch column set, returning a typed ErrInvalidPatchField for the first column
// that may not be mutated. It is the capability layer on top of the structural
// floor already checked by StructMeta.ValidatePatchColumns (unknown column, PK,
// soft-delete). The check is skipped under the write-guard bypass or a zero
// schema.
func RequireMutableColumns(ctx context.Context, schema r3.Schema, cols []string) error {
	if schema.IsZero() || r3.WriteGuardBypassed(ctx) {
		return nil
	}
	for _, c := range cols {
		if !schema.Writable(c, r3.WriteOpMutate) {
			return fmt.Errorf("%w: %q is not mutable", r3.ErrInvalidPatchField, c)
		}
	}
	return nil
}

// ManagedTimestampColumns returns the system-managed timestamp columns present on
// the model that the engine writes for the given op: created_at AND updated_at on
// Create, updated_at on Update/Patch. These are written by the system with server
// time even though they are read-only to API callers — the Creatable/Mutable caps
// mean "no caller may set this", not "never written". The soft-delete column is
// excluded here: it is managed by Delete/Restore, not by Create/Update.
func ManagedTimestampColumns(meta StructMeta, naming r3.NamingConfig, op r3.WriteOp) []string {
	naming = resolveNaming(naming)
	var out []string
	if op == r3.WriteOpCreate && slices.Contains(meta.Columns, naming.CreatedAtField) {
		out = append(out, naming.CreatedAtField)
	}
	if slices.Contains(meta.Columns, naming.UpdatedAtField) {
		out = append(out, naming.UpdatedAtField)
	}
	return out
}

// SetTimeColumns sets the named columns on the entity to t. Only time.Time and
// *time.Time fields are written; any other shape is left to the database default.
func SetTimeColumns(meta StructMeta, entityPtr any, cols []string, t time.Time) {
	if len(cols) == 0 {
		return
	}
	v := reflect.ValueOf(entityPtr).Elem()
	colToField := make(map[string]int, len(meta.Columns))
	for i, c := range meta.Columns {
		colToField[c] = meta.Fields[i]
	}
	tv := reflect.ValueOf(t)
	for _, col := range cols {
		idx, ok := colToField[col]
		if !ok {
			continue
		}
		f := v.Field(idx)
		switch {
		case !f.CanSet():
			continue
		case f.Type() == sqlTimeType:
			f.Set(tv)
		case f.Kind() == reflect.Pointer && f.Type().Elem() == sqlTimeType:
			p := reflect.New(sqlTimeType)
			p.Elem().Set(tv)
			f.Set(p)
		}
	}
}

// resolveNaming backfills any empty well-known field names with the standard
// defaults so timestamp detection is never blank.
func resolveNaming(n r3.NamingConfig) r3.NamingConfig {
	def := r3.DefaultConfig().Naming
	if n.CreatedAtField == "" {
		n.CreatedAtField = def.CreatedAtField
	}
	if n.UpdatedAtField == "" {
		n.UpdatedAtField = def.UpdatedAtField
	}
	if n.DeletedAtField == "" {
		n.DeletedAtField = def.DeletedAtField
	}
	return n
}
