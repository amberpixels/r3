package r3gorm

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"gorm.io/gorm"
)

// This file implements R3's backend-agnostic resolution of relationship ("has")
// filters: rather than emitting a subquery, a relationship filter is executed
// as a small pre-query that yields the set of matching parent keys, then
// rewritten to a plain key-set In filter — which every dialect already supports.
// It is the read-side mirror of the preload machinery in assoc.go (which walks
// the same relation metadata in the opposite direction).

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
// resolved against the DB and replaced by an equivalent In filter. It recurses
// through And/Or groups. The entity type T supplies the relation metadata.
func lowerRelationFilters[T any](ctx context.Context, db *gorm.DB, filters r3.Filters) (r3.Filters, error) {
	meta := enginesql.GetStructMeta[T]()
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

// resolveRelationFilter resolves a single relationship filter into an In filter
// on a column of the queried (parent) table.
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
		// table → the parent FKs. WHERE parent.id IN (those FKs).
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
		return r3.In(meta.PKColumn, parentKeys), nil

	case enginesql.RelBelongsTo:
		// The FK lives on the parent row: WHERE parent.fk IN (matching target PKs).
		targetKeys, err := selectRelationKeys(
			ctx, db, rel.TargetMeta.TableName, rel.TargetMeta.PKColumn, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		return r3.In(rel.FKColumn, targetKeys), nil

	case enginesql.RelHasMany:
		// The FK lives on the child row: collect the FK values of matching
		// children, then WHERE parent.id IN (those FKs).
		parentKeys, err := selectRelationKeys(
			ctx, db, rel.TargetMeta.TableName, rel.FKColumn, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		return r3.In(meta.PKColumn, parentKeys), nil

	default:
		return nil, fmt.Errorf("relationship filter: unsupported relation kind on %q", f.Relation)
	}
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
