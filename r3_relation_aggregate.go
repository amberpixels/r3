package r3

import (
	"context"
	"errors"
)

// ErrRelationAggregateNotSupported is returned by [AggregateThroughRelation]
// when the repository (or its outermost decorator) does not implement
// [RelationAggregator] — for example a non-GORM backend, which currently does
// not support relation aggregation.
var ErrRelationAggregateNotSupported = errors.New("relation aggregation not supported by this repository")

// RelationAggregator is the optional capability of aggregating an entity's
// RELATED rows through a declared relation — grouped COUNT/SUM/AVG/MIN/MAX like
// [Aggregator], but folded over the related rows (a has-many child table or a
// many-to-many join table) rather than the entity's own table.
//
// This lets counts and rollups that live in a join or child table be expressed
// on the r3 store instead of dropping to raw SQL — e.g. "members per group" over
// a membership join table.
//
// Query semantics (of the merged Query):
//   - GroupBy and Aggregates resolve against the RELATED base table: the child
//     table for has-many, the join table for many-to-many. An AggCount with no
//     field counts related rows; empty Aggregates is [ErrInvalidAggregate].
//   - Filters are interpreted as filters on the OWNER entity and restrict which
//     owners' related rows are aggregated (so a permissions Scoper's owner
//     filters apply). Empty Filters aggregates across all owners.
//   - Related rows soft-deleted in the target are excluded when the relation
//     declares a soft-delete column (see [RelationTargetSoftDelete] or a
//     tag-derived soft-delete on the target).
//   - Having, Sorts (group fields + aggregate aliases only), and Pagination
//     behave as in [Aggregator]. IncludeTrashed, Fields, Preloads, and Cursor
//     are ignored.
//
// Only has-many and many-to-many relations are aggregatable; belongs-to is not
// (each owner has at most one related row). Reach it via
// [AggregateThroughRelation] rather than asserting inner repos, so permission
// scoping is never bypassed.
type RelationAggregator interface {
	// AggregateThroughRelation folds the declared relation's related rows into
	// one AggregateRow per group.
	AggregateThroughRelation(ctx context.Context, relation string, qarg ...Query) ([]AggregateRow, error)
}

// AggregateThroughRelation aggregates repo's related rows through the named
// relation if repo (including its decorators, which all forward the capability)
// implements [RelationAggregator], and returns [ErrRelationAggregateNotSupported]
// otherwise. Like [AggregateOf], it asserts only the outermost value — never
// walking the decorator chain — so feature concerns (permission scoping in
// particular) always apply.
func AggregateThroughRelation[T any, ID comparable](
	ctx context.Context, repo Querier[T, ID], relation string, qarg ...Query,
) ([]AggregateRow, error) {
	agg, ok := repo.(RelationAggregator)
	if !ok {
		return nil, ErrRelationAggregateNotSupported
	}
	return agg.AggregateThroughRelation(ctx, relation, qarg...)
}
