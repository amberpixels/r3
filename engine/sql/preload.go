package enginesql

import (
	"context"
	"fmt"
	"reflect"

	"github.com/amberpixels/r3"
)

// RunPreloads loads the requested relations for entities (a *[]T, e.g. *[]City),
// setting the relation fields in place. Has-many: WHERE fk IN (parent PKs),
// grouped by FK into each parent's slice field. Belongs-to: WHERE pk IN (parent
// FKs), assigned to each parent's pointer field.
func RunPreloads(
	ctx context.Context,
	executor SQLExecutor,
	meta *StructMeta,
	flavor Flavor,
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
			// Skip silently; an ORM driver may handle it.
			continue
		}

		switch rel.Kind {
		case RelHasMany:
			if err := preloadHasMany(ctx, executor, flavor, sliceVal, meta, rel); err != nil {
				return fmt.Errorf("preload %s: %w", rel.FieldName, err)
			}
		case RelBelongsTo:
			if err := preloadBelongsTo(ctx, executor, flavor, sliceVal, meta, rel); err != nil {
				return fmt.Errorf("preload %s: %w", rel.FieldName, err)
			}
		}
	}

	return nil
}

// preloadHasMany loads children for a has-many relation, parent PK -> child FK
// (City.ID -> CityTranslation.CityID).
func preloadHasMany(
	ctx context.Context,
	executor SQLExecutor,
	flavor Flavor,
	sliceVal reflect.Value,
	parentMeta *StructMeta,
	rel *RelationMeta,
) error {
	n := sliceVal.Len()
	if n == 0 {
		return nil
	}

	// Collect unique parent PKs.
	pkSet := make(map[any]bool, n)
	var pkValues []any
	for i := range n {
		entity := sliceVal.Index(i)
		pkVal := entity.Field(parentMeta.Fields[parentMeta.PKField]).Interface()
		if !pkSet[pkVal] {
			pkSet[pkVal] = true
			pkValues = append(pkValues, pkVal)
		}
	}

	if len(pkValues) == 0 {
		return nil
	}

	// SELECT * FROM children WHERE fk IN (...)
	targetMeta := &rel.TargetMeta
	placeholders := flavor.Placeholders(len(pkValues), 1)
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s IN (%s)",
		ColumnsString(targetMeta.Columns),
		targetMeta.TableName,
		rel.FKColumn,
		placeholders,
	)

	rows, err := executor.QueryContext(ctx, query, pkValues...)
	if err != nil {
		return err
	}
	defer rows.Close()

	fkColIdx := -1
	for i, col := range targetMeta.Columns {
		if col == rel.FKColumn {
			fkColIdx = i
			break
		}
	}
	if fkColIdx < 0 {
		return fmt.Errorf("FK column %q not found in target table %s columns", rel.FKColumn, targetMeta.TableName)
	}

	// Scan and group children by FK value.
	grouped := make(map[any][]reflect.Value)

	for rows.Next() {
		childPtr := reflect.New(rel.TargetType)
		child := childPtr.Elem()
		dests := targetMeta.ScanDest(childPtr.Interface())
		if err := rows.Scan(dests...); err != nil {
			return err
		}
		fkVal := child.Field(targetMeta.Fields[fkColIdx]).Interface()
		grouped[fkVal] = append(grouped[fkVal], child)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Assign grouped children back to parents.
	for i := range n {
		entity := sliceVal.Index(i)
		pkVal := entity.Field(parentMeta.Fields[parentMeta.PKField]).Interface()
		children, ok := grouped[pkVal]
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

// preloadBelongsTo loads targets for a belongs-to relation, child FK -> parent PK
// (Location.CityID -> City.ID).
func preloadBelongsTo(
	ctx context.Context,
	executor SQLExecutor,
	flavor Flavor,
	sliceVal reflect.Value,
	parentMeta *StructMeta,
	rel *RelationMeta,
) error {
	n := sliceVal.Len()
	if n == 0 {
		return nil
	}

	// The parent holds the FK as a db column.
	fkColIdx := -1
	for i, col := range parentMeta.Columns {
		if col == rel.FKColumn {
			fkColIdx = i
			break
		}
	}
	if fkColIdx < 0 {
		return fmt.Errorf("FK column %q not found in parent table %s columns", rel.FKColumn, parentMeta.TableName)
	}

	// Collect unique parent FKs.
	fkSet := make(map[any]bool, n)
	var fkValues []any
	for i := range n {
		entity := sliceVal.Index(i)
		fkVal := entity.Field(parentMeta.Fields[fkColIdx]).Interface()
		if !fkSet[fkVal] {
			fkSet[fkVal] = true
			fkValues = append(fkValues, fkVal)
		}
	}

	if len(fkValues) == 0 {
		return nil
	}

	// SELECT * FROM targets WHERE pk IN (...)
	targetMeta := &rel.TargetMeta
	placeholders := flavor.Placeholders(len(fkValues), 1)
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s IN (%s)",
		ColumnsString(targetMeta.Columns),
		targetMeta.TableName,
		targetMeta.PKColumn,
		placeholders,
	)

	rows, err := executor.QueryContext(ctx, query, fkValues...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Scan and index targets by PK.
	indexed := make(map[any]reflect.Value)
	for rows.Next() {
		targetPtr := reflect.New(rel.TargetType)
		dests := targetMeta.ScanDest(targetPtr.Interface())
		if err := rows.Scan(dests...); err != nil {
			return err
		}
		target := targetPtr.Elem()
		pkVal := target.Field(targetMeta.Fields[targetMeta.PKField]).Interface()
		indexed[pkVal] = target
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Assign targets back to parents (as *City or City).
	for i := range n {
		entity := sliceVal.Index(i)
		fkVal := entity.Field(parentMeta.Fields[fkColIdx]).Interface()
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
