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

// WriteColumns returns the subset of cols the op may write under the schema. The
// capability check is skipped (all cols returned) under the write-guard bypass or
// a zero schema. The structural floor (PK never in cols; computed attributes have
// no column) is the caller's, so it can't be widened here.
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
// Patch column set, returning a typed ErrInvalidPatchField for the first
// non-mutable column - the capability layer over the structural floor from
// StructMeta.ValidatePatchColumns. Skipped under the write-guard bypass or a zero
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

// ManagedTimestampColumns returns the model's system-managed timestamp columns
// the engine writes for op: created_at AND updated_at on Create, updated_at on
// Update/Patch. The system stamps these with server time even though they are
// read-only to callers - Creatable/Mutable mean "no caller may set this", not
// "never written". Soft-delete is excluded (managed by Delete/Restore).
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
// *time.Time fields are written; any other shape is left to the DB default.
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

// resolveNaming backfills empty well-known field names with the defaults so
// timestamp detection is never blank.
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
