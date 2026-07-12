package metrics

import (
	"context"
	"time"

	"github.com/amberpixels/r3"
)

// Operation identifies which CRUD method was invoked.
type Operation string

const (
	OpCreate     Operation = "create"
	OpGet        Operation = "get"
	OpList       Operation = "list"
	OpCount      Operation = "count"
	OpAggregate  Operation = "aggregate"
	OpUpdate     Operation = "update"
	OpPatch      Operation = "patch"
	OpDelete     Operation = "delete"
	OpUpsert     Operation = "upsert"
	OpPatchWhere Operation = "patch_where"
)

// OperationContext carries everything a collector needs about a completed CRUD
// operation. Fields not relevant to a given Operation hold their zero value.
type OperationContext[T any, ID comparable] struct {
	// Operation is the CRUD method that was invoked.
	Operation Operation

	// Duration is the wall-clock time the inner operation took.
	Duration time.Duration

	// Entity is the result: post-mutation for create/update/patch, pre-deletion
	// for delete, fetched for get. Zero for list - use Entities.
	Entity T

	// Entities is the result set for List. Nil otherwise.
	Entities []T

	// OldEntity is the state before mutation (update/patch/delete).
	OldEntity T

	// HasOld reports whether OldEntity is valid.
	HasOld bool

	// EntityID is the primary key (get/delete).
	EntityID ID

	// TotalCount is List's total match count before pagination.
	TotalCount int64

	// Query is the r3.Query passed to Get or List.
	Query r3.Query

	// Fields is the field list passed to Patch.
	Fields r3.Fields

	// Err is the inner operation's error, nil on success. Most collectors fire
	// only on success; ErrorCollector uses this.
	Err error
}

// Collector emits zero or more MetricEntry values for a completed CRUD operation.
// It is called after every operation; nil means record nothing. Collectors must
// be pure - no I/O or side effects, just compute from the OperationContext.
type Collector[T any, ID comparable] interface {
	Collect(ctx context.Context, opCtx OperationContext[T, ID]) []MetricEntry
}

// CollectorFunc adapts a function to Collector for inline collectors:
//
//	metrics.CollectorFunc[Order, int64](func(ctx context.Context, opCtx metrics.OperationContext[Order, int64]) []metrics.MetricEntry {
//	    // custom logic
//	})
type CollectorFunc[T any, ID comparable] func(ctx context.Context, opCtx OperationContext[T, ID]) []MetricEntry

// Collect implements the Collector interface.
func (f CollectorFunc[T, ID]) Collect(ctx context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
	return f(ctx, opCtx)
}
