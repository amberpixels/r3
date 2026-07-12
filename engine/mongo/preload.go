package enginemongo

import (
	"context"
	"fmt"
	"reflect"

	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RunPreloads loads the requested relations for entities (a pointer to a slice,
// e.g. *[]City) via key-set $in queries, one per relation:
//
//   - has-many: query children where fk $in (parent IDs), group by FK, assign to
//     the parent's slice field.
//   - belongs-to: query targets where _id $in (parent FKs), assign to the
//     parent's pointer field.
func RunPreloads(
	ctx context.Context,
	db *mongo.Database,
	meta *StructMeta,
	entitiesPtr any,
	preloads r3.Preloads,
) error {
	if len(preloads) == 0 || len(meta.Relations) == 0 {
		return nil
	}

	relMap := make(map[string]*RelationMeta, len(meta.Relations))
	for i := range meta.Relations {
		relMap[meta.Relations[i].FieldName] = &meta.Relations[i]
	}

	sliceVal := reflect.ValueOf(entitiesPtr).Elem()
	if sliceVal.Kind() != reflect.Slice {
		return fmt.Errorf("RunPreloads: entitiesPtr must be a pointer to a slice, got %T", entitiesPtr)
	}

	for _, preload := range preloads {
		rel, ok := relMap[preload.GetName()]
		if !ok {
			continue
		}

		switch rel.Kind {
		case RelHasMany:
			if err := preloadHasMany(ctx, db, sliceVal, meta, rel); err != nil {
				return fmt.Errorf("preload %s: %w", rel.FieldName, err)
			}
		case RelBelongsTo:
			if err := preloadBelongsTo(ctx, db, sliceVal, meta, rel); err != nil {
				return fmt.Errorf("preload %s: %w", rel.FieldName, err)
			}
		}
	}

	return nil
}

// preloadHasMany loads child records for a has-many relation.
func preloadHasMany(
	ctx context.Context,
	db *mongo.Database,
	sliceVal reflect.Value,
	parentMeta *StructMeta,
	rel *RelationMeta,
) error {
	n := sliceVal.Len()
	if n == 0 {
		return nil
	}

	idSet := make(map[any]bool, n)
	var idValues []any
	for i := range n {
		entity := sliceVal.Index(i)
		idVal := entity.Field(parentMeta.FieldIndices[parentMeta.IDFieldIdx]).Interface()
		if !idSet[idVal] {
			idSet[idVal] = true
			idValues = append(idValues, idVal)
		}
	}

	if len(idValues) == 0 {
		return nil
	}

	targetMeta := &rel.TargetMeta
	coll := db.Collection(targetMeta.CollectionName)

	filter := bson.D{{Key: rel.FKField, Value: bson.D{{Key: inOp, Value: idValues}}}}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	fkFieldIdx := -1
	for i, f := range targetMeta.Fields {
		if f == rel.FKField {
			fkFieldIdx = i
			break
		}
	}
	if fkFieldIdx < 0 {
		return fmt.Errorf("FK field %q not found in target collection %s", rel.FKField, targetMeta.CollectionName)
	}

	// Group scanned children by FK value.
	grouped := make(map[any][]reflect.Value)
	for cursor.Next(ctx) {
		childPtr := reflect.New(rel.TargetType)
		if err := cursor.Decode(childPtr.Interface()); err != nil {
			return err
		}
		child := childPtr.Elem()
		fkVal := child.Field(targetMeta.FieldIndices[fkFieldIdx]).Interface()
		grouped[fkVal] = append(grouped[fkVal], child)
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	for i := range n {
		entity := sliceVal.Index(i)
		idVal := entity.Field(parentMeta.FieldIndices[parentMeta.IDFieldIdx]).Interface()
		children, ok := grouped[idVal]
		if !ok || len(children) == 0 {
			continue
		}

		childSlice := reflect.MakeSlice(entity.Field(rel.FieldIndex).Type(), len(children), len(children))
		for j, child := range children {
			childSlice.Index(j).Set(child)
		}
		entity.Field(rel.FieldIndex).Set(childSlice)
	}

	return nil
}

// preloadBelongsTo loads parent records for a belongs-to relation.
func preloadBelongsTo(
	ctx context.Context,
	db *mongo.Database,
	sliceVal reflect.Value,
	parentMeta *StructMeta,
	rel *RelationMeta,
) error {
	n := sliceVal.Len()
	if n == 0 {
		return nil
	}

	// The FK lives on the parent (the belongs-to side).
	fkFieldIdx := -1
	for i, f := range parentMeta.Fields {
		if f == rel.FKField {
			fkFieldIdx = i
			break
		}
	}
	if fkFieldIdx < 0 {
		return fmt.Errorf("FK field %q not found in parent collection %s", rel.FKField, parentMeta.CollectionName)
	}

	fkSet := make(map[any]bool, n)
	var fkValues []any
	for i := range n {
		entity := sliceVal.Index(i)
		fkVal := entity.Field(parentMeta.FieldIndices[fkFieldIdx]).Interface()
		if !fkSet[fkVal] {
			fkSet[fkVal] = true
			fkValues = append(fkValues, fkVal)
		}
	}

	if len(fkValues) == 0 {
		return nil
	}

	targetMeta := &rel.TargetMeta
	coll := db.Collection(targetMeta.CollectionName)

	filter := bson.D{{Key: targetMeta.IDField, Value: bson.D{{Key: inOp, Value: fkValues}}}}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Index scanned targets by ID.
	indexed := make(map[any]reflect.Value)
	for cursor.Next(ctx) {
		targetPtr := reflect.New(rel.TargetType)
		if err := cursor.Decode(targetPtr.Interface()); err != nil {
			return err
		}
		target := targetPtr.Elem()
		idVal := target.Field(targetMeta.FieldIndices[targetMeta.IDFieldIdx]).Interface()
		indexed[idVal] = target
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	for i := range n {
		entity := sliceVal.Index(i)
		fkVal := entity.Field(parentMeta.FieldIndices[fkFieldIdx]).Interface()
		target, ok := indexed[fkVal]
		if !ok {
			continue
		}

		field := entity.Field(rel.FieldIndex)
		if field.Kind() == reflect.Pointer {
			ptr := reflect.New(rel.TargetType)
			ptr.Elem().Set(target)
			field.Set(ptr)
		} else {
			field.Set(target)
		}
	}

	return nil
}
