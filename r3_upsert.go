package r3

import (
	"context"
	"fmt"
)

// Upserter is the opt-in upsert capability: insert the entity, or update it in
// place on conflict. The write analogue of [Aggregator] - reached through
// [UpsertOf] (which returns [ErrUpsertNotSupported] for a backend that lacks it)
// so decorators always apply, and kept out of core [Commander] so adding it
// breaks no existing backend.
type Upserter[T any, ID comparable] interface {
	// Upsert inserts entity, or updates the colliding row when it conflicts on
	// the conflict target (the primary key by default; see [OnConflict]). It
	// returns the stored row after the write.
	//
	// Implemented by engine/sql (every flavor), gorm, bun and mongo; engine/file
	// and gopg return [ErrUpsertNotSupported]. [IncrementOnConflict] works on all
	// four, except a gorm repo on an unrecognized dialect
	// ([ErrUpsertIncrementNotSupported]).
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
	// mutable columns of the entity (a full "replace"), minus IncrementFields.
	UpdateFields Fields
	// IncrementFields are the columns the conflict branch ADDS to rather than
	// overwrites: the stored value plus the incoming one. On insert the column
	// simply takes the incoming value, so a counter's first write stores it as-is.
	// Increment by a negative value to decrement.
	//
	// A column is overwritten or incremented, never both - see [UpsertSpec.Validate].
	// The default (empty UpdateFields) full replace excludes these columns, so a
	// counter is not silently overwritten by the very upsert that increments it.
	IncrementFields Fields
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

// IncrementOnConflict marks columns the conflict branch adds to rather than
// overwrites - the counter case, where the stored value matters:
//
//	r3.UpsertOf(ctx, repo, usage,
//	    r3.OnConflict("model", "day"),
//	    r3.IncrementOnConflict(r3.NewFieldSpec("tokens")),
//	)
//
// On insert the column takes the incoming value; on conflict it takes stored plus
// incoming. Pass a negative value to decrement. A column here must not also appear
// in [UpdateOnConflict] ([ErrUpsertIncrementConflict]).
//
// Supported by engine/sql (every flavor), gorm (on a recognized dialect), bun and
// mongo; see [ErrUpsertIncrementNotSupported].
func IncrementOnConflict(fields ...*FieldSpec) UpsertOption {
	return func(s *UpsertSpec) { s.IncrementFields = fields }
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

// Validate checks the structural rule the two write sets share: a column is
// overwritten or incremented on conflict, never both. Engines call it before
// lowering an upsert, so a malformed spec means the same thing on every backend
// rather than whichever the driver happens to apply last.
func (s UpsertSpec) Validate() error {
	if len(s.UpdateFields) == 0 || len(s.IncrementFields) == 0 {
		return nil
	}
	overwritten := make(map[string]bool, len(s.UpdateFields))
	for _, f := range s.UpdateFields {
		overwritten[f.String()] = true
	}
	for _, f := range s.IncrementFields {
		if overwritten[f.String()] {
			return fmt.Errorf("%w: %q", ErrUpsertIncrementConflict, f.String())
		}
	}
	return nil
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
