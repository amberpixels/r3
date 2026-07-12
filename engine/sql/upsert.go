package enginesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/amberpixels/r3"
)

var _ r3.Upserter[any, any] = &BaseCRUD[any, any]{}

// Upsert inserts entity, or updates the colliding row on a conflict against the
// conflict target (the PK by default), via [Flavor.UpsertClause] (Postgres/SQLite
// ON CONFLICT ... DO UPDATE, MySQL ON DUPLICATE KEY UPDATE).
//
// The insert branch obeys Creatable and stamps created_at/updated_at; the update
// branch obeys Mutable (a restricted UpdateOnConflict set is rejected like Patch)
// and bumps updated_at. The write-guard bypass skips the capability check as in
// Create/Patch. The stored row comes back via RETURNING where supported, else a
// re-fetch by the conflict target.
func (r *BaseCRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	spec := r3.NewUpsertSpec(opts...)

	conflictCols := spec.ConflictColumns
	if len(conflictCols) == 0 {
		conflictCols = []string{r.Meta.PKColumn}
	}

	updateCols, err := r.upsertUpdateColumns(ctx, &entity, spec)
	if err != nil {
		return entity, err
	}
	insertCols := r.upsertInsertColumns(ctx, &entity)
	vals := r.Meta.FieldValuesForColumns(entity, insertCols)

	base := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) %s",
		r.Meta.TableName,
		ColumnsString(insertCols),
		r.Flavor.Placeholders(len(insertCols), 1),
		r.Flavor.UpsertClause(conflictCols, updateCols),
	)

	// RETURNING gives the stored row, but only when a DO UPDATE runs: DO NOTHING
	// returns no row on collision, so fall through to the re-fetch (as does MySQL,
	// which lacks RETURNING).
	if r.Flavor.SupportsRETURNING && len(updateCols) > 0 {
		query := base + " RETURNING " + ColumnsString(r.Meta.Columns)
		dests := r.Meta.ScanDest(&entity)
		if err := r.Executor.QueryRowContext(ctx, query, vals...).Scan(dests...); err != nil {
			return entity, err
		}
		return entity, nil
	}

	if _, err := r.Executor.ExecContext(ctx, base, vals...); err != nil {
		return entity, err
	}
	return r.fetchByColumns(ctx, conflictCols, r.Meta.FieldValuesForColumns(entity, conflictCols))
}

// upsertInsertColumns returns the columns the INSERT branch writes: createColumns
// (Creatable + managed created_at/updated_at) plus a non-zero caller PK, so an
// upsert keyed by a non-generated PK (e.g. a string settings key) inserts under
// it. A zero PK is left out so a serial/auto-increment PK is still DB-generated.
func (r *BaseCRUD[T, ID]) upsertInsertColumns(ctx context.Context, entityPtr *T) []string {
	cols := r.createColumns(ctx, entityPtr)
	if pk := r.Meta.PKValue(*entityPtr); pk != nil && !reflect.ValueOf(pk).IsZero() {
		return append([]string{r.Meta.PKColumn}, cols...)
	}
	return cols
}

// upsertUpdateColumns returns the columns the on-conflict update branch writes:
// every mutable column plus managed updated_at (full replace) by default, or -
// with an explicit UpdateOnConflict set - the Patch-validated columns
// (ValidatePatchColumns + RequireMutableColumns) plus managed updated_at.
func (r *BaseCRUD[T, ID]) upsertUpdateColumns(
	ctx context.Context, entityPtr *T, spec r3.UpsertSpec,
) ([]string, error) {
	if len(spec.UpdateFields) == 0 {
		return r.updateColumns(ctx, entityPtr), nil
	}
	cols := FieldsToColumns(spec.UpdateFields)
	cols, err := r.Meta.ValidatePatchColumns(cols)
	if err != nil {
		return nil, err
	}
	if err := RequireMutableColumns(ctx, r.Schema, cols); err != nil {
		return nil, err
	}
	cols = append(cols, r.stampManagedTimestamps(ctx, entityPtr, cols, r3.WriteOpMutate)...)
	return cols, nil
}

// fetchByColumns selects the single row matching cols = vals into a fresh entity,
// normalizing "no rows" to r3.ErrNotFound. It is the upsert re-fetch for backends
// without a usable RETURNING (MySQL, or the DO NOTHING branch).
func (r *BaseCRUD[T, ID]) fetchByColumns(ctx context.Context, cols []string, vals []any) (T, error) {
	var entity T
	whereParts := make([]string, len(cols))
	for i, c := range cols {
		whereParts[i] = r.Flavor.WhereEq(c, i+1)
	}
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		ColumnsString(r.Meta.Columns),
		r.Meta.TableName,
		strings.Join(whereParts, " AND "),
	)
	dests := r.Meta.ScanDest(&entity)
	err := r.Executor.QueryRowContext(ctx, query, vals...).Scan(dests...)
	if errors.Is(err, sql.ErrNoRows) {
		return entity, r3.ErrNotFound
	}
	return entity, err
}
