package metrics

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/amberpixels/r3"
)

// Metric names for entity-level collectors.
const (
	// MetricEntityFieldChange tracks which fields change most often on updates.
	MetricEntityFieldChange = "entity.field_change"

	// MetricEntityLifecycle tracks the duration from creation to deletion.
	MetricEntityLifecycle = "entity.lifecycle"

	// MetricEntityPatchSize tracks how many fields are patched at once.
	MetricEntityPatchSize = "entity.patch_size"
)

// labelField is the metric label key naming the entity field a metric refers to.
const labelField = "field"

// FieldChangeCollector emits one "entity.field_change" entry per changed field,
// labeled {"field": "<name>"}. Patch uses the Fields list; Update/Upsert detect
// changes via a shallow reflect comparison, so it needs the old entity (HasOld).
func FieldChangeCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil {
			return nil
		}
		if !opCtx.HasOld {
			return nil
		}

		switch opCtx.Operation {
		case OpPatch, OpPatchWhere:
			// Patch/PatchWhere name exactly the fields written.
			fieldNames := r3.FieldsToStrings(opCtx.Fields)
			entries := make([]MetricEntry, 0, len(fieldNames))
			for _, name := range fieldNames {
				entries = append(entries, MetricEntry{
					MetricName: MetricEntityFieldChange,
					Value:      1,
					Labels:     Labels{labelField: name},
				})
			}
			return entries

		case OpUpdate, OpUpsert:
			changed := detectChangedFields(opCtx.OldEntity, opCtx.Entity)
			entries := make([]MetricEntry, 0, len(changed))
			for _, name := range changed {
				entries = append(entries, MetricEntry{
					MetricName: MetricEntityFieldChange,
					Value:      1,
					Labels:     Labels{labelField: name},
				})
			}
			return entries

		case OpCreate, OpGet, OpList, OpCount, OpAggregate, OpDelete:
			return nil
		}

		return nil
	})
}

// detectChangedFields shallow-compares exported fields and returns the names of
// those whose values differ.
func detectChangedFields[T any](old, cur T) []string {
	oldV := reflect.ValueOf(old)
	curV := reflect.ValueOf(cur)
	if oldV.Kind() == reflect.Pointer {
		oldV = oldV.Elem()
	}
	if curV.Kind() == reflect.Pointer {
		curV = curV.Elem()
	}
	if oldV.Kind() != reflect.Struct || curV.Kind() != reflect.Struct {
		return nil
	}

	var changed []string
	t := oldV.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		oldF := oldV.Field(i)
		curF := curV.Field(i)
		if !reflect.DeepEqual(oldF.Interface(), curF.Interface()) {
			changed = append(changed, field.Name)
		}
	}
	return changed
}

// LifecycleCollector emits one "entity.lifecycle" entry on Delete, valued as
// hours since creation. createdAtFunc extracts the creation timestamp; pass nil
// (or return a zero time) and the collector is a no-op.
func LifecycleCollector[T any, ID comparable](createdAtFunc func(T) time.Time) Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil || opCtx.Operation != OpDelete {
			return nil
		}
		if !opCtx.HasOld || createdAtFunc == nil {
			return nil
		}

		createdAt := createdAtFunc(opCtx.OldEntity)
		if createdAt.IsZero() {
			return nil
		}

		hours := time.Since(createdAt).Hours()
		return []MetricEntry{{
			MetricName: MetricEntityLifecycle,
			Value:      hours,
		}}
	})
}

// PatchSizeCollector emits one "entity.patch_size" entry on Patch, valued as the
// number of fields patched at once.
func PatchSizeCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil || opCtx.Operation != OpPatch {
			return nil
		}

		return []MetricEntry{{
			MetricName: MetricEntityPatchSize,
			Value:      float64(len(opCtx.Fields)),
			Labels:     Labels{"field_count": strconv.Itoa(len(opCtx.Fields))},
		}}
	})
}
