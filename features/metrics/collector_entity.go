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

// FieldChangeCollector tracks which fields change most often on Update and Patch.
// Emits one "entity.field_change" entry per changed field.
// Labels: {"field": "<field_name>"}.
//
// For Patch operations, the fields are taken from the Fields list.
// For Update operations, this does a shallow reflect-based comparison to detect changes.
// Requires the old entity to be available (IDFunc must be set).
func FieldChangeCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil {
			return nil
		}
		if !opCtx.HasOld {
			return nil
		}

		switch opCtx.Operation {
		case OpPatch:
			// For Patch, we know exactly which fields were patched.
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

		case OpUpdate:
			// For Update, detect which fields actually changed via reflection.
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

		case OpCreate, OpGet, OpList, OpDelete:
			return nil
		}

		return nil
	})
}

// detectChangedFields performs a shallow comparison of exported fields
// and returns the names (snake_case) of fields whose values differ.
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

// LifecycleCollector tracks how long entities live (creation to deletion).
// Emits one "entity.lifecycle" entry on Delete with value = hours since creation.
//
// The createdAtFunc parameter extracts the creation timestamp from the entity.
// If the entity has no creation time, pass nil and this collector is a no-op.
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

// PatchSizeCollector tracks how many fields are patched at once.
// Emits one "entity.patch_size" entry on Patch with value = number of fields.
//
// Helps understand usage patterns and API ergonomics.
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
