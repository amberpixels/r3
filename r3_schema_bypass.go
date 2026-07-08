package r3

import "context"

// The write guard enforces the schema's write capabilities (Creatable/Mutable)
// at the engine seam. It is always on; callers opt out explicitly, per call.
//
// The bypass is the audited system/worker door (see the schema design doc, §2.5
// and §6): a worker that owns a feed-synced, user-immutable column may write it
// even though Mutable=false for all API callers. Because the marker rides on the
// context, it flows down through the history/metrics decorators first — so the
// write is still audited — and only the engine's capability check is skipped.
//
// The structural floor (§2.4) is independent and never bypassed: a computed
// attribute has no column to write, and the PK is identity. The worst a bypass
// can do is write a real column the public contract would have refused.

type writeGuardBypassKey struct{}

// WithoutWriteGuard returns a context that skips the engine's write-capability
// enforcement for writes made with it. Use it for audited system/worker writes
// of a user-immutable column; prefer the SystemWriter wrapper for ergonomics.
//
// It does not bypass the structural floor (computed/PK remain unwritable), and
// it does not bypass any decorator (history/metrics still record the write).
func WithoutWriteGuard(ctx context.Context) context.Context {
	return context.WithValue(ctx, writeGuardBypassKey{}, true)
}

// WriteGuardBypassed reports whether the context carries the write-guard bypass.
// Engines consult it before enforcing Creatable/Mutable.
func WriteGuardBypassed(ctx context.Context) bool {
	v, ok := ctx.Value(writeGuardBypassKey{}).(bool)
	return ok && v
}

// SystemWriter wraps the top of a repository (decorator) chain so its write
// methods inject WithoutWriteGuard into the context before delegating. The full
// chain still runs — history/metrics/soft-delete all see the write — but the
// engine skips the Creatable/Mutable check. Reads and the structural floor are
// unaffected.
//
//	r3.SystemWriter(repo).Update(ctx, feedRow) // writes a user-immutable column, audited
func SystemWriter[T any, ID comparable](repo CRUD[T, ID]) CRUD[T, ID] {
	return systemWriter[T, ID]{inner: repo}
}

// systemWriter is the SystemWriter decorator. Reads delegate unchanged; writes
// inject the bypass marker.
type systemWriter[T any, ID comparable] struct {
	inner CRUD[T, ID]
}

func (w systemWriter[T, ID]) Get(ctx context.Context, id ID, q ...Query) (T, error) {
	return w.inner.Get(ctx, id, q...)
}

func (w systemWriter[T, ID]) List(ctx context.Context, q ...Query) ([]T, int64, error) {
	return w.inner.List(ctx, q...)
}

func (w systemWriter[T, ID]) Count(ctx context.Context, q ...Query) (int64, error) {
	return w.inner.Count(ctx, q...)
}

func (w systemWriter[T, ID]) Aggregate(ctx context.Context, q ...Query) ([]AggregateRow, error) {
	agg, ok := w.inner.(Aggregator)
	if !ok {
		return nil, ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, q...)
}

func (w systemWriter[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	return w.inner.Create(WithoutWriteGuard(ctx), entity)
}

func (w systemWriter[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	return w.inner.Update(WithoutWriteGuard(ctx), entity)
}

func (w systemWriter[T, ID]) Patch(ctx context.Context, entity T, fields Fields) (T, error) {
	return w.inner.Patch(WithoutWriteGuard(ctx), entity, fields)
}

func (w systemWriter[T, ID]) Delete(ctx context.Context, id ID) error {
	return w.inner.Delete(ctx, id)
}
