package metrics

import (
	"context"
	"time"

	"github.com/amberpixels/r3"
)

// Operation identifies which CRUD method was invoked.
type Operation string

const (
	OpCreate    Operation = "create"
	OpGet       Operation = "get"
	OpList      Operation = "list"
	OpCount     Operation = "count"
	OpAggregate Operation = "aggregate"
	OpUpdate    Operation = "update"
	OpPatch     Operation = "patch"
	OpDelete    Operation = "delete"
)

// OperationContext carries contextual information about a CRUD operation.
// It is passed to collectors so they can emit metrics based on what happened.
type OperationContext[T any, ID comparable] struct {
	// Operation is the CRUD method that was invoked.
	Operation Operation

	// Duration is the wall-clock time the inner CRUD operation took.
	Duration time.Duration

	// Entity is the result entity (after mutation for create/update/patch,
	// before deletion for delete, fetched entity for get).
	// For list operations, this is the zero value — use Entities instead.
	Entity T

	// Entities is the result set for List operations. Nil for other operations.
	Entities []T

	// OldEntity is the entity state before mutation (for update/patch/delete).
	// Zero value for create/get/list.
	OldEntity T

	// HasOld indicates whether OldEntity is valid.
	HasOld bool

	// ID is the entity primary key (for get/delete). Zero value for create/list.
	EntityID ID

	// TotalCount is the total matching count from List (before pagination). Zero for other ops.
	TotalCount int64

	// Query is the r3.Query passed to Get or List. Empty for mutations.
	Query r3.Query

	// Fields is the field list passed to Patch. Nil for other operations.
	Fields r3.Fields

	// Err is the error returned by the inner CRUD operation (nil on success).
	// Most collectors only fire on success, but ErrorCollector uses this.
	Err error
}

// Collector defines what metrics to emit for CRUD operations.
// Each collector is called after every CRUD operation and returns zero or more
// MetricEntry values to record. Returning nil means "don't record anything."
//
// Collectors receive a rich OperationContext with all the information they need.
// They should NOT perform I/O or side effects — just compute and return entries.
type Collector[T any, ID comparable] interface {
	Collect(ctx context.Context, opCtx OperationContext[T, ID]) []MetricEntry
}

// CollectorFunc is a function adapter for Collector, enabling inline collectors:
//
//	metrics.CollectorFunc[Order, int64](func(ctx context.Context, opCtx metrics.OperationContext[Order, int64]) []metrics.MetricEntry {
//	    // custom logic
//	})
type CollectorFunc[T any, ID comparable] func(ctx context.Context, opCtx OperationContext[T, ID]) []MetricEntry

// Collect implements the Collector interface.
func (f CollectorFunc[T, ID]) Collect(ctx context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
	return f(ctx, opCtx)
}
