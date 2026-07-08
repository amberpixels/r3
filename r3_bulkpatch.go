package r3

import "context"

// BulkPatcher is the optional bulk-update capability of a repository: set the
// named fields (to the entity's values) on every row matching filters, returning
// the affected-row count. It is the multi-row analogue of Patch — same partial,
// by-[Fields] semantics — but selects rows by [Filters] instead of by primary
// key.
//
// Like [Upserter] it is opt-in and reached through [PatchWhereOf], so feature
// decorators (permission scoping in particular) always apply. A backend that
// does not implement it surfaces [ErrBulkPatchNotSupported] from [PatchWhereOf]
// rather than panicking.
type BulkPatcher[T any, ID comparable] interface {
	// PatchWhere writes fields (taken from entity) on every row matching
	// filters and returns the number of rows affected.
	PatchWhere(ctx context.Context, filters Filters, entity T, fields Fields) (int64, error)
}

// PatchWhereOf runs a bulk conditional update against repo if it (including any
// decorators, which forward the capability) implements [BulkPatcher], and
// returns [ErrBulkPatchNotSupported] otherwise. Like [UpsertOf] it asserts only
// the outermost value so decorator concerns — the permissions scope confinement
// in particular — always apply.
func PatchWhereOf[T any, ID comparable](
	ctx context.Context, repo Commander[T, ID], filters Filters, entity T, fields Fields,
) (int64, error) {
	bp, ok := repo.(BulkPatcher[T, ID])
	if !ok {
		return 0, ErrBulkPatchNotSupported
	}
	return bp.PatchWhere(ctx, filters, entity, fields)
}
