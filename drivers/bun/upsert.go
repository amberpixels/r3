package r3bun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

var _ r3.Upserter[any, any] = &BunCRUD[any, any]{}

// Upsert inserts entity, or on a conflict with the target columns updates the
// named columns in place, then returns the stored row. It implements
// [r3.Upserter] via SQL `INSERT ... ON CONFLICT (...) DO UPDATE SET ... RETURNING *`.
//
// The conflict target defaults to the primary key; the update set defaults to
// every column except the primary key, the conflict columns, and created_at
// (never overwritten). An explicit UpdateOnConflict set is validated like Patch
// (unknown/PK/soft-delete columns are rejected). Timestamp bumping on the update
// branch is left to the database (default/trigger), matching this driver's
// create/update behavior. Requires a dialect with ON CONFLICT and RETURNING
// (Postgres, SQLite).
func (r *BunCRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	spec := r3.NewUpsertSpec(opts...)
	meta := enginesql.GetStructMeta[T]()

	conflictCols := spec.ConflictColumns
	if len(conflictCols) == 0 {
		conflictCols = []string{meta.PKColumn}
	}
	updateCols := r3.FieldsToStrings(spec.UpdateFields)
	if len(updateCols) > 0 {
		// An explicit UpdateOnConflict set mirrors Patch: unknown, primary-key,
		// and soft-delete columns are rejected with typed errors.
		var err error
		if updateCols, err = meta.ValidatePatchColumns(updateCols); err != nil {
			return entity, err
		}
	} else {
		updateCols = defaultUpsertUpdateColumns(meta, r.Config.Naming, conflictCols)
	}

	target, err := quoteIdentList(conflictCols)
	if err != nil {
		return entity, err
	}

	// With nothing left to update (the conflict target covers every updatable
	// column) DO UPDATE would render an empty SET; degrade to DO NOTHING and
	// re-fetch the surviving row by the conflict target.
	if len(updateCols) == 0 {
		if _, err := r.db.NewInsert().Model(&entity).
			On("CONFLICT (?) DO NOTHING", bun.Safe(target)).Exec(ctx); err != nil {
			return entity, err
		}
		return r.getByColumns(ctx, meta, conflictCols, entity)
	}

	q := r.db.NewInsert().Model(&entity).On("CONFLICT (?) DO UPDATE", bun.Safe(target))
	for _, col := range updateCols {
		if err := r3.ValidateIdentifier(col); err != nil {
			return entity, fmt.Errorf("r3/bun: upsert update column %q: %w", col, err)
		}
		q = q.Set("? = EXCLUDED.?", bun.Ident(col), bun.Ident(col))
	}

	if _, err := q.Returning("*").Exec(ctx); err != nil {
		return entity, err
	}
	return entity, nil
}

// getByColumns fetches the single row matching cols (values read from entity),
// normalizing "no rows" to r3.ErrNotFound. It is the upsert re-fetch for the
// DO NOTHING branch, where RETURNING yields no row on a collision.
func (r *BunCRUD[T, ID]) getByColumns(
	ctx context.Context, meta enginesql.StructMeta, cols []string, entity T,
) (T, error) {
	var out T
	vals := meta.FieldValuesForColumns(entity, cols)
	q := r.db.NewSelect().Model(&out)
	for i, c := range cols {
		q = q.Where("? = ?", bun.Ident(c), vals[i])
	}
	if err := q.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, r3.ErrNotFound
		}
		return out, err
	}
	return out, nil
}

// quoteIdentList validates and double-quotes each column, joining them for a
// conflict target list. Columns originate from struct tags / field specs, but
// validating keeps the assembled raw fragment injection-safe.
func quoteIdentList(cols []string) (string, error) {
	parts := make([]string, len(cols))
	for i, c := range cols {
		if err := r3.ValidateIdentifier(c); err != nil {
			return "", fmt.Errorf("r3/bun: upsert conflict column %q: %w", c, err)
		}
		parts[i] = `"` + c + `"`
	}
	return strings.Join(parts, ", "), nil
}

// defaultUpsertUpdateColumns is the "full replace" set: every column except the
// primary key, the conflict target, and created_at.
func defaultUpsertUpdateColumns(meta enginesql.StructMeta, naming r3.NamingConfig, conflictCols []string) []string {
	skip := map[string]bool{meta.PKColumn: true}
	if naming.CreatedAtField != "" {
		skip[naming.CreatedAtField] = true
	}
	for _, c := range conflictCols {
		skip[c] = true
	}
	out := make([]string, 0, len(meta.Columns))
	for _, c := range meta.Columns {
		if !skip[c] {
			out = append(out, c)
		}
	}
	return out
}
