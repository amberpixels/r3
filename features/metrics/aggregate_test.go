package metrics_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/metrics"
	"github.com/expectto/be"
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
	be.NoError(t, err)
	be.RequireThat(t, rows, be.HaveLength(2))

	be.RequireThat(t, seen, be.HaveLength(1))
	be.AssertThat(t, seen[0].Operation, be.Eq(metrics.OpAggregate))
	be.AssertThat(t, seen[0].TotalCount, be.Eq(int64(2)))
	be.AssertThat(t, len(seen[0].Query.GroupBy), be.Eq(1),
		"expected the query to be carried into the operation context")
	be.AssertThat(t, len(inner.lastQuery.Aggregates), be.Eq(1),
		"expected the query to reach the inner repo")
}

func TestCRUD_AggregateInnerWithoutSupport(t *testing.T) {
	inner := newMemoryCRUD() // no Aggregate method
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store)

	_, err := r3.AggregateOf(context.Background(), repo, r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	be.ErrorIs(t, err, r3.ErrAggregateNotSupported)
}
