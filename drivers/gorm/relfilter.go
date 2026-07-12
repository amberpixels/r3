package r3gorm

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"gorm.io/gorm"
)

// This file resolves relationship ("has") filters backend-agnostically: instead of
// emitting a subquery, a relationship filter runs as a small pre-query yielding the
// matching parent keys, then is rewritten to a plain key-set In filter that every
// dialect supports. It is the read-side mirror of the preload machinery in assoc.go
// (which walks the same relation metadata in the opposite direction).

// hasRelationFilters reports whether any filter in the tree is a relationship
// filter, so the common (relation-free) path stays a no-op.
func hasRelationFilters(filters r3.Filters) bool {
	for _, f := range filters {
		if f == nil {
			continue
		}
		if f.Relation != "" {
			return true
		}
		if hasRelationFilters(f.And) || hasRelationFilters(f.Or) {
			return true
		}
	}
	return false
}

// lowerRelationFilters returns a copy of filters with every relationship filter
// resolved against the DB and replaced by an equivalent In/NotIn filter. It
// recurses through And/Or groups. meta (the repository's own StructMeta, which
// already includes any explicitly-declared relations) supplies the relation
// metadata.
func lowerRelationFilters(
	ctx context.Context, db *gorm.DB, meta enginesql.StructMeta, filters r3.Filters,
) (r3.Filters, error) {
	return lowerFilterGroup(ctx, db, meta, filters)
}

func lowerFilterGroup(
	ctx context.Context, db *gorm.DB, meta enginesql.StructMeta, group r3.Filters,
) (r3.Filters, error) {
	out := make(r3.Filters, len(group))
	for i, f := range group {
		lowered, err := lowerRelationFilter(ctx, db, meta, f)
		if err != nil {
			return nil, err
		}
		out[i] = lowered
	}
	return out, nil
}

func lowerRelationFilter(
	ctx context.Context, db *gorm.DB, meta enginesql.StructMeta, f *r3.FilterSpec,
) (*r3.FilterSpec, error) {
	switch {
	case f == nil:
		return nil, nil //nolint:nilnil // a nil child stays nil; callers tolerate it
	case f.Relation != "":
		return resolveRelationFilter(ctx, db, meta, f)
	case len(f.And) > 0:
		children, err := lowerFilterGroup(ctx, db, meta, f.And)
		if err != nil {
			return nil, err
		}
		return &r3.FilterSpec{And: children}, nil
	case len(f.Or) > 0:
		children, err := lowerFilterGroup(ctx, db, meta, f.Or)
		if err != nil {
			return nil, err
		}
		return &r3.FilterSpec{Or: children}, nil
	default:
		return f, nil
	}
}

// resolveRelationFilter resolves a single relationship filter into a key-set
// filter on a column of the queried (parent) table: an In filter for Has, or a
// NotIn (anti-join) for HasNo. Because "col IN ()" matches nothing and
// "col NOT IN ()" matches everything, the empty-key set falls out correctly for
// both directions with no special-casing.
func resolveRelationFilter(
	ctx context.Context, db *gorm.DB, meta enginesql.StructMeta, f *r3.FilterSpec,
) (*r3.FilterSpec, error) {
	rel, ok := findRelation(meta, f.Relation)
	if !ok {
		return nil, fmt.Errorf("relationship filter: unknown relation %q on %s", f.Relation, meta.TableName)
	}

	switch rel.Kind {
	case enginesql.RelManyToMany:
		// Two hops: related rows matching the inner filter → their PKs → the join
		// table → the parent FKs. WHERE parent.id [NOT] IN (those FKs).
		targetKeys, err := selectRelationKeys(
			ctx, db, rel.TargetMeta.TableName, rel.TargetMeta.PKColumn, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		parentKeys := []any{}
		if len(targetKeys) > 0 {
			parentKeys, err = selectRelationKeys(
				ctx, db, rel.JoinTable, rel.FKColumn, r3.Filters{r3.In(rel.RefColumn, targetKeys)},
			)
			if err != nil {
				return nil, err
			}
		}
		return keySetFilter(meta.PKColumn, parentKeys, f.RelationNegate), nil

	case enginesql.RelBelongsTo:
		// The FK lives on the parent row: WHERE parent.fk [NOT] IN (matching
		// target PKs). For the negated case a NULL FK means "has no related row",
		// so it must be included — NOT IN alone would drop it (NULL NOT IN (...)
		// is never true) — hence the extra OR fk IS NULL.
		targetKeys, err := selectRelationKeys(
			ctx, db, rel.TargetMeta.TableName, rel.TargetMeta.PKColumn, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		if f.RelationNegate {
			return r3.Or(r3.NotIn(rel.FKColumn, targetKeys), r3.Eq(rel.FKColumn, nil)), nil
		}
		return r3.In(rel.FKColumn, targetKeys), nil

	case enginesql.RelHasMany:
		// The FK lives on the child row: collect the FK values of matching
		// children, then WHERE parent.id [NOT] IN (those FKs).
		parentKeys, err := selectRelationKeys(
			ctx, db, rel.TargetMeta.TableName, rel.FKColumn, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		return keySetFilter(meta.PKColumn, parentKeys, f.RelationNegate), nil

	default:
		return nil, fmt.Errorf("relationship filter: unsupported relation kind on %q", f.Relation)
	}
}

// keySetFilter builds the parent-side key-set filter: NotIn for an anti-join
// (HasNo), In otherwise. The parent key column is never NULL, so no NULL guard
// is needed here (unlike the belongs-to FK case).
func keySetFilter(column string, keys []any, negate bool) *r3.FilterSpec {
	if negate {
		return r3.NotIn(column, keys)
	}
	return r3.In(column, keys)
}

// selectRelationKeys runs `SELECT DISTINCT <col> FROM <table> WHERE <inner>` and
// returns the distinct, non-null values. An empty inner filter selects all rows.
func selectRelationKeys(
	ctx context.Context, db *gorm.DB, table, col string, inner r3.Filters,
) ([]any, error) {
	clauses, err := r3sql.FiltersToSQL(inner)
	if err != nil {
		return nil, fmt.Errorf("translate relation filter: %w", err)
	}

	// Alias the selected column to a stable key so the scanned map is predictable.
	q := db.WithContext(ctx).Table(table).Select(col + " AS k")
	for _, c := range clauses {
		q = q.Where(c.Clause, c.Args...)
	}

	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("resolve relation keys: %w", err)
	}

	keys := make([]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		v := row["k"]
		if v == nil {
			continue
		}
		sk := fmt.Sprintf("%v", v)
		if _, ok := seen[sk]; ok {
			continue
		}
		seen[sk] = struct{}{}
		keys = append(keys, v)
	}
	return keys, nil
}

func findRelation(meta enginesql.StructMeta, name string) (enginesql.RelationMeta, bool) {
	for _, rel := range meta.Relations {
		if rel.FieldName == name {
			return rel, true
		}
	}
	return enginesql.RelationMeta{}, false
}
