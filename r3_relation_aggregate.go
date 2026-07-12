package r3

import (
	"context"
	"errors"
)

// ErrRelationAggregateNotSupported is returned by [AggregateThroughRelation] when
// the repository does not implement [RelationAggregator] (relation aggregation is
// GORM-only today).
var ErrRelationAggregateNotSupported = errors.New("relation aggregation not supported by this repository")

// RelationAggregator is the opt-in capability of aggregating an entity's RELATED
// rows through a declared relation: grouped COUNT/SUM/AVG/MIN/MAX like
// [Aggregator], but folded over the related rows (a has-many child table or a
// many-to-many join table) rather than the entity's own - e.g. "members per
// group" over a membership join table, without dropping to raw SQL.
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

// AggregateThroughRelation folds repo's related rows through the named relation,
// or returns [ErrRelationAggregateNotSupported] if repo does not implement
// [RelationAggregator]. Like [AggregateOf], it asserts only the outermost value -
// never the decorator chain - so permission scoping is never bypassed.
func AggregateThroughRelation[T any, ID comparable](
	ctx context.Context, repo Querier[T, ID], relation string, qarg ...Query,
) ([]AggregateRow, error) {
	agg, ok := repo.(RelationAggregator)
	if !ok {
		return nil, ErrRelationAggregateNotSupported
	}
	return agg.AggregateThroughRelation(ctx, relation, qarg...)
}
