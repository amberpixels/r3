package r3

import "context"

// Upserter is the opt-in upsert capability: insert the entity, or update it in
// place on conflict. The write analogue of [Aggregator] - reached through
// [UpsertOf] (which returns [ErrUpsertNotSupported] for a backend that lacks it)
// so decorators always apply, and kept out of core [Commander] so adding it
// breaks no existing backend.
type Upserter[T any, ID comparable] interface {
	// Upsert inserts entity, or updates the colliding row when it conflicts on
	// the conflict target (the primary key by default; see [OnConflict]). It
	// returns the stored row after the write.
	Upsert(ctx context.Context, entity T, opts ...UpsertOption) (T, error)
}

// UpsertSpec is the resolved configuration of an Upsert: the conflict target and
// the columns overwritten on conflict. Build it from [UpsertOption]s via
// [NewUpsertSpec]; engines and decorators read it to shape the write.
type UpsertSpec struct {
	// ConflictColumns is the conflict target — the columns whose collision
	// triggers the update branch. Empty means the primary key.
	ConflictColumns []string
	// UpdateFields are the columns overwritten on conflict. Empty means all
	// mutable columns of the entity (a full "replace").
	UpdateFields Fields
}

// UpsertOption configures an [UpsertSpec].
type UpsertOption func(*UpsertSpec)

// OnConflict sets the conflict-target columns. With no option the target is the
// primary key.
func OnConflict(cols ...string) UpsertOption {
	return func(s *UpsertSpec) { s.ConflictColumns = cols }
}

// UpdateOnConflict restricts which columns are written on conflict. With no
// option every mutable column is overwritten.
func UpdateOnConflict(fields ...*FieldSpec) UpsertOption {
	return func(s *UpsertSpec) { s.UpdateFields = fields }
}

// NewUpsertSpec resolves the given options into a concrete [UpsertSpec]. Engines
// implementing [Upserter] call this to interpret the caller's options.
func NewUpsertSpec(opts ...UpsertOption) UpsertSpec {
	var s UpsertSpec
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}
	return s
}

// UpsertOf runs an upsert against repo, or returns [ErrUpsertNotSupported] if it
// does not implement [Upserter]. Like [AggregateOf], it asserts only the
// outermost value - never the decorator chain - so permission checks always apply.
func UpsertOf[T any, ID comparable](
	ctx context.Context, repo Commander[T, ID], entity T, opts ...UpsertOption,
) (T, error) {
	up, ok := repo.(Upserter[T, ID])
	if !ok {
		var zero T
		return zero, ErrUpsertNotSupported
	}
	return up.Upsert(ctx, entity, opts...)
}
