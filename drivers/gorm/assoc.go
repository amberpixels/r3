package r3gorm

import (
	"reflect"

	"github.com/amberpixels/r3"
	enginesql "github.com/amberpixels/r3/engine/sql"
	"gorm.io/gorm"
)

// splitPreloads separates preloads into R3-managed (have RelationMeta with r3 tags)
// and GORM-managed (no r3 relation tag — use GORM's native Preload).
func splitPreloads[T any](preloads r3.Preloads) (r3Managed []enginesql.RelationMeta, gormManaged r3.Preloads) {
	if len(preloads) == 0 {
		return nil, nil
	}

	meta := enginesql.GetStructMeta[T]()
	r3RelMap := make(map[string]enginesql.RelationMeta, len(meta.Relations))
	for _, rel := range meta.Relations {
		r3RelMap[rel.FieldName] = rel
	}

	for _, p := range preloads {
		if rel, ok := r3RelMap[p.GetName()]; ok {
			r3Managed = append(r3Managed, rel)
		} else {
			gormManaged = append(gormManaged, p)
		}
	}

	return r3Managed, gormManaged
}

// runR3Preloads loads relations for a slice of entities using direct SQL,
// based on R3 relation metadata. Modifies entities in-place.
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
	// Collect parent PKs
	parentIDs := collectPKs(entities, meta)
	if len(parentIDs) == 0 {
		return nil
	}

	// Query join table: SELECT fk, ref FROM join_table WHERE fk IN (?)
	type joinRow struct {
		FK  int64 `gorm:"column:fk"`
		Ref int64 `gorm:"column:ref"`
	}
	var rows []joinRow
	if err := db.Raw(
		"SELECT "+rel.FKColumn+" as fk, "+rel.RefColumn+" as ref FROM "+rel.JoinTable+" WHERE "+rel.FKColumn+" IN (?)",
		parentIDs,
	).Scan(&rows).Error; err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	// Collect unique child IDs
	childIDSet := make(map[int64]bool)
	for _, row := range rows {
		childIDSet[row.Ref] = true
	}
	childIDs := make([]int64, 0, len(childIDSet))
	for id := range childIDSet {
		childIDs = append(childIDs, id)
	}

	// Query children: SELECT * FROM children WHERE pk IN (?)
	childSlice := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), 0, len(childIDs))
	childPtr := reflect.New(childSlice.Type())
	childPtr.Elem().Set(childSlice)
	if err := db.Table(rel.TargetMeta.TableName).Where(rel.TargetMeta.PKColumn+" IN (?)", childIDs).Find(childPtr.Interface()).Error; err != nil {
		return err
	}
	loadedChildren := childPtr.Elem()

	// Index children by PK
	childByPK := make(map[int64]reflect.Value)
	for i := range loadedChildren.Len() {
		child := loadedChildren.Index(i)
		pkVal := child.Field(rel.TargetMeta.Fields[rel.TargetMeta.PKField]).Interface()
		if id, ok := pkVal.(int64); ok {
			childByPK[id] = child
		}
	}

	// Build parent→children mapping from join rows
	parentToChildren := make(map[int64][]reflect.Value)
	for _, row := range rows {
		if child, ok := childByPK[row.Ref]; ok {
			parentToChildren[row.FK] = append(parentToChildren[row.FK], child)
		}
	}

	// Assign to entities
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		parentPK := ev.Field(meta.Fields[meta.PKField]).Interface().(int64)
		children := parentToChildren[parentPK]
		childSlice := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), len(children), len(children))
		for j, child := range children {
			childSlice.Index(j).Set(child)
		}
		ev.Field(rel.FieldIndex).Set(childSlice)
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
	if err := db.Table(rel.TargetMeta.TableName).Where(rel.FKColumn+" IN (?)", parentIDs).Find(childPtr.Interface()).Error; err != nil {
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

	// Group children by FK value
	parentToChildren := make(map[int64][]reflect.Value)
	for i := range loadedChildren.Len() {
		child := loadedChildren.Index(i)
		fkVal := child.Field(fkFieldIdx).Interface().(int64)
		parentToChildren[fkVal] = append(parentToChildren[fkVal], child)
	}

	// Assign to entities
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		parentPK := ev.Field(meta.Fields[meta.PKField]).Interface().(int64)
		children := parentToChildren[parentPK]
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

	fkIDs := make(map[int64]bool)
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		fkField := ev.Field(fkFieldIdx)
		if fkField.Kind() == reflect.Pointer {
			if fkField.IsNil() {
				continue
			}
			fkField = fkField.Elem()
		}
		if id, ok := fkField.Interface().(int64); ok && id != 0 {
			fkIDs[id] = true
		}
	}

	if len(fkIDs) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(fkIDs))
	for id := range fkIDs {
		ids = append(ids, id)
	}

	// Query targets
	targetSlice := reflect.MakeSlice(reflect.SliceOf(rel.TargetType), 0, 0)
	targetPtr := reflect.New(targetSlice.Type())
	targetPtr.Elem().Set(targetSlice)
	if err := db.Table(rel.TargetMeta.TableName).Where(rel.TargetMeta.PKColumn+" IN (?)", ids).Find(targetPtr.Interface()).Error; err != nil {
		return err
	}
	loadedTargets := targetPtr.Elem()

	// Index by PK
	targetByPK := make(map[int64]reflect.Value)
	for i := range loadedTargets.Len() {
		t := loadedTargets.Index(i)
		pkVal := t.Field(rel.TargetMeta.Fields[rel.TargetMeta.PKField]).Interface().(int64)
		targetByPK[pkVal] = t
	}

	// Assign to entities
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		fkField := ev.Field(fkFieldIdx)
		if fkField.Kind() == reflect.Pointer {
			if fkField.IsNil() {
				continue
			}
			fkField = fkField.Elem()
		}
		if id, ok := fkField.Interface().(int64); ok {
			if target, exists := targetByPK[id]; exists {
				relField := ev.Field(rel.FieldIndex)
				if relField.Kind() == reflect.Pointer {
					ptr := reflect.New(rel.TargetType)
					ptr.Elem().Set(target)
					relField.Set(ptr)
				} else {
					relField.Set(target)
				}
			}
		}
	}

	return nil
}

// collectPKs extracts PK values from a slice of entities.
func collectPKs[T any](entities []T, meta enginesql.StructMeta) []int64 {
	ids := make([]int64, 0, len(entities))
	for i := range entities {
		ev := reflect.ValueOf(&entities[i]).Elem()
		if pk, ok := ev.Field(meta.Fields[meta.PKField]).Interface().(int64); ok {
			ids = append(ids, pk)
		}
	}
	return ids
}
