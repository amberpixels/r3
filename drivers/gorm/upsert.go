package r3gorm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
)

var _ r3.Upserter[any, any] = &GormCRUD[any, any]{}

// Upsert inserts entity, or updates the colliding row on a conflict at the conflict
// target (the primary key by default), via GORM's clause.OnConflict.
//
// The insert branch obeys Creatable and stamps created_at/updated_at (shared
// writeOmit path); the update branch obeys Mutable and bumps updated_at. The write
// guard bypass (r3.WithoutWriteGuard) is honored. The stored row is re-fetched by
// the conflict target and returned, so the result reflects the update, defaults,
// and triggers even on a collision.
//
// Upsert is row-level and does not sync associations; use Create/Update for
// entities carrying related rows.
func (r *GormCRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	spec := r3.NewUpsertSpec(opts...)
	if err := spec.Validate(); err != nil {
		return entity, err
	}

	incrementCols, err := r.upsertIncrementColumns(ctx, spec)
	if err != nil {
		return entity, err
	}
	updateCols, err := r.upsertUpdateColumns(ctx, &entity, spec, incrementCols)
	if err != nil {
		return entity, err
	}

	conflictCols := spec.ConflictColumns
	if len(conflictCols) == 0 {
		conflictCols = []string{r.meta.PKColumn}
	}

	onConflict := clause.OnConflict{Columns: toClauseColumns(conflictCols)}
	switch {
	case len(updateCols)+len(incrementCols) == 0:
		onConflict.DoNothing = true
	case len(incrementCols) == 0:
		onConflict.DoUpdates = clause.AssignmentColumns(updateCols)
	default:
		assignments, err := incrementAssignments(
			r.db,
			gormTableName(r.db, &entity, r.meta.TableName),
			updateCols,
			incrementCols,
		)
		if err != nil {
			return entity, err
		}
		onConflict.DoUpdates = assignments
	}

	db := r.db.WithContext(ctx)
	// Insert branch obeys Creatable: omit non-creatable columns so the DB fills
	// their defaults, while managed timestamps are stamped and written.
	if omit := r.writeOmit(ctx, &entity, r3.WriteOpCreate); len(omit) > 0 {
		db = db.Omit(omit...)
	}
	if err := db.Clauses(onConflict).Create(&entity).Error; err != nil {
		return entity, err
	}

	return r.getByColumns(ctx, conflictCols, entity)
}

// incrementAssignments renders the conflict branch when some columns accumulate:
// plain overwrites stay `col = EXCLUDED.col`, incremented ones become
// `col = col + EXCLUDED.col`. The right-hand reference to the proposed row is
// dialect-specific, so it comes from the same gormFlavor mapping the bucket path
// uses - and an unrecognized dialect degrades loudly there for the same reason,
// rather than emitting SQL that silently overwrites a counter.
func incrementAssignments(db *gorm.DB, table string, updateCols, incrementCols []string) (clause.Set, error) {
	flavor := gormFlavor(db)

	set := make(clause.Set, 0, len(updateCols)+len(incrementCols))
	set = append(set, clause.AssignmentColumns(updateCols)...)
	for _, col := range incrementCols {
		if err := r3.ValidateIdentifier(col); err != nil {
			return nil, fmt.Errorf("r3/gorm: upsert increment column %q: %w", col, err)
		}
		expr, err := flavor.UpsertIncrementExpr(table, col)
		if err != nil {
			return nil, err
		}
		set = append(set, clause.Assignment{
			Column: clause.Column{Name: col},
			Value:  gorm.Expr(expr),
		})
	}
	return set, nil
}

// gormTableName asks GORM what table it will write to, so an accumulating
// assignment qualifies the stored value with the name actually in scope. The
// reflected meta name is only a fallback: it is the snake-case plural of the Go
// type, which a model with its own TableName() or a custom NamingStrategy does
// not use.
func gormTableName(db *gorm.DB, entity any, fallback string) string {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(entity); err == nil && stmt.Table != "" {
		return stmt.Table
	}
	return fallback
}

// upsertIncrementColumns resolves the columns the conflict branch adds to,
// validated like any other written column (Patch rules + Mutable).
func (r *GormCRUD[T, ID]) upsertIncrementColumns(
	ctx context.Context, spec r3.UpsertSpec,
) ([]string, error) {
	if len(spec.IncrementFields) == 0 {
		return nil, nil
	}
	cols, err := r.meta.ValidatePatchColumns(r3.FieldsToStrings(spec.IncrementFields))
	if err != nil {
		return nil, err
	}
	if err := enginesql.RequireMutableColumns(ctx, r.schema, cols); err != nil {
		return nil, err
	}
	return cols, nil
}

// upsertUpdateColumns resolves the columns the update branch writes. With no
// UpdateOnConflict option: every mutable column plus managed updated_at (a full
// replace). With an explicit set it mirrors Patch: reject PK/soft-delete/unknown
// and non-mutable columns, then append managed updated_at.
//
// Incremented columns are dropped from the default full-replace set, so a counter
// is not overwritten by the very upsert that increments it.
func (r *GormCRUD[T, ID]) upsertUpdateColumns(
	ctx context.Context, entityPtr *T, spec r3.UpsertSpec, incrementCols []string,
) ([]string, error) {
	if len(spec.UpdateFields) == 0 {
		cols := enginesql.WriteColumns(ctx, r.schema, r.meta.NonPKColumns(), r3.WriteOpMutate)
		cols = slices.DeleteFunc(slices.Clone(cols), func(c string) bool {
			return slices.Contains(incrementCols, c)
		})
		return append(cols, r.stampManaged(ctx, entityPtr, cols, r3.WriteOpMutate)...), nil
	}
	cols := r3.FieldsToStrings(spec.UpdateFields)
	cols, err := r.meta.ValidatePatchColumns(cols)
	if err != nil {
		return nil, err
	}
	if err := enginesql.RequireMutableColumns(ctx, r.schema, cols); err != nil {
		return nil, err
	}
	return append(cols, r.stampManaged(ctx, entityPtr, cols, r3.WriteOpMutate)...), nil
}

// stampManaged sets server time on the op's managed timestamp columns
// (created_at/updated_at) not already listed by the caller, and returns them.
// No-op under a write-guard bypass or zero schema, mirroring engine/sql's
// stampManagedTimestamps.
func (r *GormCRUD[T, ID]) stampManaged(
	ctx context.Context, entityPtr *T, callerCols []string, op r3.WriteOp,
) []string {
	if r.schema.IsZero() || r3.WriteGuardBypassed(ctx) {
		return nil
	}
	var inject []string
	for _, c := range enginesql.ManagedTimestampColumns(r.meta, r.Config.Naming, op) {
		if !slices.Contains(callerCols, c) {
			inject = append(inject, c)
		}
	}
	enginesql.SetTimeColumns(r.meta, entityPtr, inject, time.Now())
	return inject
}

// getByColumns fetches the single row matching cols (read from entity) and
// returns it, normalizing GORM's not-found to r3.ErrNotFound. It is the upsert
// re-fetch by conflict target.
func (r *GormCRUD[T, ID]) getByColumns(ctx context.Context, cols []string, entity T) (T, error) {
	var out T
	q := r.db.WithContext(ctx)
	for _, c := range cols {
		vals := r.meta.FieldValuesForColumns(entity, []string{c})
		if len(vals) == 1 {
			q = q.Where(clause.Eq{Column: c, Value: vals[0]})
		}
	}
	if err := q.First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, r3.ErrNotFound
		}
		return out, err
	}
	return out, nil
}

// toClauseColumns converts column names to GORM clause.Column targets.
func toClauseColumns(cols []string) []clause.Column {
	out := make([]clause.Column, len(cols))
	for i, c := range cols {
		out[i] = clause.Column{Name: c}
	}
	return out
}
