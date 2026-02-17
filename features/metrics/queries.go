package metrics

import (
	"github.com/amberpixels/r3"
)

// Query builder field names -- matching the db/bson tags on MetricRecord.
var (
	fieldRecordType = r3.NewFieldSpec("record_type")
	fieldRecordID   = r3.NewFieldSpec("record_id")
	fieldMetricName = r3.NewFieldSpec("metric_name")
	fieldBucket     = r3.NewFieldSpec("bucket")
	fieldCreatedAt  = r3.NewFieldSpec("created_at")
)

// QueryByType builds a Query that retrieves all metrics for a given entity type
// within a time range, sorted by created_at descending.
func QueryByType(recordType string, tr TimeRange) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.Fop(fieldCreatedAt, r3.OperatorGte, tr.From),
				r3.Fop(fieldCreatedAt, r3.OperatorLte, tr.To),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryByEntity builds a Query that retrieves all metrics for a specific entity instance.
func QueryByEntity(recordType, recordID string, tr TimeRange) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
				r3.Fop(fieldCreatedAt, r3.OperatorGte, tr.From),
				r3.Fop(fieldCreatedAt, r3.OperatorLte, tr.To),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryByMetric builds a Query that retrieves a specific metric for a given entity type.
func QueryByMetric(recordType, metricName string, tr TimeRange) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldMetricName, metricName),
				r3.Fop(fieldCreatedAt, r3.OperatorGte, tr.From),
				r3.Fop(fieldCreatedAt, r3.OperatorLte, tr.To),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryByEntityMetric builds a Query for a specific metric on a specific entity.
func QueryByEntityMetric(recordType, recordID, metricName string, tr TimeRange) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
				r3.F(fieldMetricName, metricName),
				r3.Fop(fieldCreatedAt, r3.OperatorGte, tr.From),
				r3.Fop(fieldCreatedAt, r3.OperatorLte, tr.To),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryByBucket builds a Query that retrieves metrics for a specific time bucket.
func QueryByBucket(recordType, metricName, bucket string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldMetricName, metricName),
				r3.F(fieldBucket, bucket),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}
