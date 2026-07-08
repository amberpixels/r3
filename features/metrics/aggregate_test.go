package metrics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/metrics"
)

// aggMemoryCRUD extends the in-memory mock with a canned Aggregate so the
// decorator's forwarding + instrumentation can be observed.
type aggMemoryCRUD struct {
	*memoryCRUD
	lastQuery r3.Query
}

func (m *aggMemoryCRUD) Aggregate(_ context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	if len(qarg) > 0 {
		m.lastQuery = qarg[0]
	}
	return []r3.AggregateRow{
		{"status": "new", "n": int64(2)},
		{"status": "done", "n": int64(1)},
	}, nil
}

func TestCRUD_AggregateRecordsMetrics(t *testing.T) {
	inner := &aggMemoryCRUD{memoryCRUD: newMemoryCRUD()}
	store := newMemoryMetricsCRUD()

	var seen []metrics.OperationContext[Order, int64]
	spy := metrics.CollectorFunc[Order, int64](
		func(_ context.Context, opCtx metrics.OperationContext[Order, int64]) []metrics.MetricEntry {
			seen = append(seen, opCtx)
			return nil
		})

	repo := metrics.WithMetrics[Order, int64](inner, store,
		metrics.WithIDFunc[Order, int64](orderIDFunc),
		metrics.WithCollectors[Order, int64](spy),
	)

	q := r3.Query{
		GroupBy:    r3.GroupBy("status"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	}
	rows, err := r3.AggregateOf(context.Background(), repo, q)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected the inner rows back, got %d", len(rows))
	}

	if len(seen) != 1 {
		t.Fatalf("expected 1 recorded operation, got %d", len(seen))
	}
	if seen[0].Operation != metrics.OpAggregate {
		t.Errorf("expected OpAggregate, got %q", seen[0].Operation)
	}
	if seen[0].TotalCount != 2 {
		t.Errorf("expected TotalCount 2 (grouped rows), got %d", seen[0].TotalCount)
	}
	if len(seen[0].Query.GroupBy) != 1 {
		t.Errorf("expected the query to be carried into the operation context")
	}
	if len(inner.lastQuery.Aggregates) != 1 {
		t.Errorf("expected the query to reach the inner repo")
	}
}

func TestCRUD_AggregateInnerWithoutSupport(t *testing.T) {
	inner := newMemoryCRUD() // no Aggregate method
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store)

	_, err := r3.AggregateOf(context.Background(), repo, r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if !errors.Is(err, r3.ErrAggregateNotSupported) {
		t.Fatalf("expected ErrAggregateNotSupported, got %v", err)
	}
}
