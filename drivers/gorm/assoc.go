package r3gorm

import (
	"fmt"
	"reflect"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"gorm.io/gorm"
)

// pkKey stringifies a primary/foreign key so keys of any comparable type group
// consistently - including across the int/int64 mismatch between a struct field
// and the same value scanned back from the DB.
func pkKey(v any) string { return fmt.Sprintf("%v", v) }

// splitPreloads partitions preloads into R3-managed (have RelationMeta from r3
// tags) and GORM-managed (no r3 relation tag; use GORM's native Preload).
func splitPreloads[T any](preloads r3.Preloads) ([]enginesql.RelationMeta, r3.Preloads) {
	if len(preloads) == 0 {
		return nil, nil
	}

	meta := enginesql.GetStructMeta[T]()
	r3RelMap := make(map[string]enginesql.RelationMeta, len(meta.Relations))
	for _, rel := range meta.Relations {
		r3RelMap[rel.FieldName] = rel
	}

	var r3Managed []enginesql.RelationMeta
	var gormManaged r3.Preloads
	for _, p := range preloads {
		if rel, ok := r3RelMap[p.GetName()]; ok {
			r3Managed = append(r3Managed, rel)
		} else {
			gormManaged = append(gormManaged, p)
		}
	}

	return r3Managed, gormManaged
}

// runR3Preloads loads R3-managed relations for entities via direct SQL, in place.
func runR3Preloads[T any](db *gorm.DB, entities []T, rels []enginesql.RelationMeta) error {
	meta := enginesql.GetStructMeta[T]()

	for _, rel := range rels {
		switch rel.Kind {
		case enginesql.RelManyToMany:
			if err := preloadM2M(db, entities, meta, rel); err != nil {
				return err
			}
		case enginesql.RelHasMany:
			if err := preloadHasMany(db, entities, meta, rel); err != nil {
				return err
			}
		case enginesql.RelBelongsTo:
			if err := preloadBelongsTo(db, entities, meta, rel); err != nil {
				return err
			}
		}
	}

	return nil
}

// preloadM2M loads a many-to-many relation via the join table.
func preloadM2M[T any](db *gorm.DB, entities []T, meta enginesql.StructMeta, rel enginesql.RelationMeta) error {
	parentIDs := collectPKs(entities, meta)
	if len(parentIDs) == 0 {
		return nil
	}

	// Query join table: SELECT fk, ref FROM join_table WHERE fk IN (?).
	// Scan into maps so keys of any type (int, string/UUID, ...) are preserved.
	var rows []map[string]any
	if err := db.Raw(
		"SELECT "+rel.FKColumn+" as fk, "+rel.RefColumn+" as ref FROM "+rel.JoinTable+" WHERE "+rel.FKColumn+" IN (?)",
		parentIDs,
	).Scan(&rows).Error; err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	// Collect unique child ref values (raw, for the IN query).
	childIDs := make([]any, 0, len(rows))
	seenChild := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		k := pkKey(row["ref"])
		if _, ok := seenChild[k]; ok {
			continue
		}
		seenChild[k] = struct{}{}
		childIDs = append(childIDs, row["ref"])
	}

	// Query children: SELECT * FROM children WHERE pk IN (?)
	childSlice := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), 0, len(childIDs))
	childPtr := reflect.New(childSlice.Type())
	childPtr.Elem().Set(childSlice)
	if err := db.Table(rel.TargetMeta.TableName).
		Where(rel.TargetMeta.PKColumn+" IN (?)", childIDs).
		Find(childPtr.Interface()).
		Error; err != nil {
		return err
	}
	loadedChildren := childPtr.Elem()

	// Index children by PK (string-normalized).
	pkFieldIdx := rel.TargetMeta.Fields[rel.TargetMeta.PKField]
	childByPK := make(map[string]reflect.Value, loadedChildren.Len())
	for i := range loadedChildren.Len() {
		child := loadedChildren.Index(i)
		childByPK[pkKey(child.Field(pkFieldIdx).Interface())] = child
	}

	// Build parent→children mapping from join rows.
	parentToChildren := make(map[string][]reflect.Value)
	for _, row := range rows {
		if child, ok := childByPK[pkKey(row["ref"])]; ok {
			fkKey := pkKey(row["fk"])
			parentToChildren[fkKey] = append(parentToChildren[fkKey], child)
		}
	}

	parentPKIdx := meta.Fields[meta.PKField]
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		children := parentToChildren[pkKey(ev.Field(parentPKIdx).Interface())]
		cs := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), len(children), len(children))
		for j, child := range children {
			cs.Index(j).Set(child)
		}
		ev.Field(rel.FieldIndex).Set(cs)
	}

	return nil
}

// preloadHasMany loads a has-many relation by querying children by FK.
func preloadHasMany[T any](db *gorm.DB, entities []T, meta enginesql.StructMeta, rel enginesql.RelationMeta) error {
	parentIDs := collectPKs(entities, meta)
	if len(parentIDs) == 0 {
		return nil
	}

	// Query children: SELECT * FROM children WHERE fk IN (?)
	childSlice := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), 0, 0)
	childPtr := reflect.New(childSlice.Type())
	childPtr.Elem().Set(childSlice)
	if err := db.Table(rel.TargetMeta.TableName).
		Where(rel.FKColumn+" IN (?)", parentIDs).
		Find(childPtr.Interface()).
		Error; err != nil {
		return err
	}
	loadedChildren := childPtr.Elem()

	// Find FK field index on child struct
	fkFieldIdx := -1
	for i, col := range rel.TargetMeta.Columns {
		if col == rel.FKColumn {
			fkFieldIdx = rel.TargetMeta.Fields[i]
			break
		}
	}
	if fkFieldIdx < 0 {
		return nil
	}

	// Group children by FK value (string-normalized; FK may be a pointer).
	parentToChildren := make(map[string][]reflect.Value)
	for i := range loadedChildren.Len() {
		child := loadedChildren.Index(i)
		fk := child.Field(fkFieldIdx)
		if fk.Kind() == reflect.Pointer {
			if fk.IsNil() {
				continue
			}
			fk = fk.Elem()
		}
		key := pkKey(fk.Interface())
		parentToChildren[key] = append(parentToChildren[key], child)
	}

	parentPKIdx := meta.Fields[meta.PKField]
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		children := parentToChildren[pkKey(ev.Field(parentPKIdx).Interface())]
		cs := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), len(children), len(children))
		for j, child := range children {
			cs.Index(j).Set(child)
		}
		ev.Field(rel.FieldIndex).Set(cs)
	}

	return nil
}

// preloadBelongsTo loads a belongs-to relation by querying targets by PK.
func preloadBelongsTo[T any](db *gorm.DB, entities []T, meta enginesql.StructMeta, rel enginesql.RelationMeta) error {
	// Collect FK values from parent entities
	fkFieldIdx := -1
	for i, col := range meta.Columns {
		if col == rel.FKColumn {
			fkFieldIdx = meta.Fields[i]
			break
		}
	}
	if fkFieldIdx < 0 {
		return nil
	}

	// Collect unique, non-zero FK values (raw, for the IN query).
	fkIDs := make([]any, 0, len(entities))
	seen := make(map[string]struct{}, len(entities))
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		fkField := ev.Field(fkFieldIdx)
		if fkField.Kind() == reflect.Pointer {
			if fkField.IsNil() {
				continue
			}
			fkField = fkField.Elem()
		}
		if fkField.IsZero() {
			continue
		}
		k := pkKey(fkField.Interface())
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		fkIDs = append(fkIDs, fkField.Interface())
	}

	if len(fkIDs) == 0 {
		return nil
	}

	// Query targets
	targetSlice := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), 0, 0)
	targetPtr := reflect.New(targetSlice.Type())
	targetPtr.Elem().Set(targetSlice)
	if err := db.Table(rel.TargetMeta.TableName).
		Where(rel.TargetMeta.PKColumn+" IN (?)", fkIDs).
		Find(targetPtr.Interface()).
		Error; err != nil {
		return err
	}
	loadedTargets := targetPtr.Elem()

	// Index by PK (string-normalized).
	pkFieldIdx := rel.TargetMeta.Fields[rel.TargetMeta.PKField]
	targetByPK := make(map[string]reflect.Value, loadedTargets.Len())
	for i := range loadedTargets.Len() {
		t := loadedTargets.Index(i)
		targetByPK[pkKey(t.Field(pkFieldIdx).Interface())] = t
	}

	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		fkField := ev.Field(fkFieldIdx)
		if fkField.Kind() == reflect.Pointer {
			if fkField.IsNil() {
				continue
			}
			fkField = fkField.Elem()
		}
		target, exists := targetByPK[pkKey(fkField.Interface())]
		if !exists {
			continue
		}
		relField := ev.Field(rel.FieldIndex)
		if relField.Kind() == reflect.Pointer {
			ptr := reflect.New(rel.TargetType)
			ptr.Elem().Set(target)
			relField.Set(ptr)
		} else {
			relField.Set(target)
		}
	}

	return nil
}

// collectPKs extracts unique PK values from a slice of entities. Values are
// returned in their native type for use as query arguments; keys of any
// comparable type are supported.
func collectPKs[T any](entities []T, meta enginesql.StructMeta) []any {
	ids := make([]any, 0, len(entities))
	seen := make(map[string]struct{}, len(entities))
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		pk := ev.Field(meta.Fields[meta.PKField]).Interface()
		k := pkKey(pk)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		ids = append(ids, pk)
	}
	return ids
}
