package sqlbase

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/amberpixels/r3"
)

// RunPreloads executes preload queries for the given entities based on the requested
// preload names and the relation metadata in StructMeta.
//
// For has-many relations: collects parent PKs, queries child table WHERE fk IN (...),
// groups results by FK, and assigns them to the parent's slice field.
//
// For belongs-to relations: collects FK values from parents, queries target table
// WHERE pk IN (...), and assigns results to the parent's pointer field.
//
// entities must be a pointer to a slice (e.g. *[]City). The function modifies
// entities in place by setting their relation fields.
func RunPreloads(
	ctx context.Context,
	db *sql.DB,
	meta *StructMeta,
	flavor Flavor,
	entitiesPtr any,
	preloads r3.Preloads,
) error {
	if len(preloads) == 0 || len(meta.Relations) == 0 {
		return nil
	}

	// Build a map of relation name -> RelationMeta for quick lookup
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
			// No matching relation found — skip silently (ORM drivers may handle it)
			continue
		}

		switch rel.Kind {
		case RelHasMany:
			if err := preloadHasMany(ctx, db, flavor, sliceVal, meta, rel); err != nil {
				return fmt.Errorf("preload %s: %w", rel.FieldName, err)
			}
		case RelBelongsTo:
			if err := preloadBelongsTo(ctx, db, flavor, sliceVal, meta, rel); err != nil {
				return fmt.Errorf("preload %s: %w", rel.FieldName, err)
			}
		}
	}

	return nil
}

// preloadHasMany loads child records for a has-many relation.
// Parent PK -> Child FK (e.g. City.ID -> CityTranslation.CityID)
func preloadHasMany(
	ctx context.Context,
	db *sql.DB,
	flavor Flavor,
	sliceVal reflect.Value,
	parentMeta *StructMeta,
	rel *RelationMeta,
) error {
	n := sliceVal.Len()
	if n == 0 {
		return nil
	}

	// Collect unique parent PK values
	pkSet := make(map[any]bool, n)
	var pkValues []any
	for i := 0; i < n; i++ {
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

	// Query child table: SELECT * FROM children WHERE fk IN (...)
	targetMeta := &rel.TargetMeta
	placeholders := flavor.Placeholders(len(pkValues), 1)
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s IN (%s)",
		ColumnsString(targetMeta.Columns),
		targetMeta.TableName,
		rel.FKColumn,
		placeholders,
	)

	rows, err := db.QueryContext(ctx, query, pkValues...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Find the FK column index in the target meta
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

	// Scan results and group by FK value
	// key: FK value (as interface{}), value: slice of child entities (reflect.Value)
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

	// Assign grouped children back to parent entities
	for i := 0; i < n; i++ {
		entity := sliceVal.Index(i)
		pkVal := entity.Field(parentMeta.Fields[parentMeta.PKField]).Interface()
		children, ok := grouped[pkVal]
		if !ok || len(children) == 0 {
			continue
		}

		// Build a slice of the correct type and assign
		childSlice := reflect.MakeSlice(entity.Field(rel.FieldIndex).Type(), len(children), len(children))
		for j, child := range children {
			childSlice.Index(j).Set(child)
		}
		entity.Field(rel.FieldIndex).Set(childSlice)
	}

	return nil
}

// preloadBelongsTo loads parent records for a belongs-to relation.
// Child FK -> Parent PK (e.g. Location.CityID -> City.ID)
func preloadBelongsTo(
	ctx context.Context,
	db *sql.DB,
	flavor Flavor,
	sliceVal reflect.Value,
	parentMeta *StructMeta,
	rel *RelationMeta,
) error {
	n := sliceVal.Len()
	if n == 0 {
		return nil
	}

	// Find the FK column index in the parent (the entity that has the FK field as a db column)
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

	// Collect unique FK values from the parent entities
	fkSet := make(map[any]bool, n)
	var fkValues []any
	for i := 0; i < n; i++ {
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

	// Query target table: SELECT * FROM targets WHERE pk IN (...)
	targetMeta := &rel.TargetMeta
	placeholders := flavor.Placeholders(len(fkValues), 1)
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s IN (%s)",
		ColumnsString(targetMeta.Columns),
		targetMeta.TableName,
		targetMeta.PKColumn,
		placeholders,
	)

	rows, err := db.QueryContext(ctx, query, fkValues...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Scan results and index by PK
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

	// Assign targets back to parent entities
	for i := 0; i < n; i++ {
		entity := sliceVal.Index(i)
		fkVal := entity.Field(parentMeta.Fields[fkColIdx]).Interface()
		target, ok := indexed[fkVal]
		if !ok {
			continue
		}

		field := entity.Field(rel.FieldIndex)
		if field.Kind() == reflect.Pointer {
			// Set as pointer: *City
			ptr := reflect.New(rel.TargetType)
			ptr.Elem().Set(target)
			field.Set(ptr)
		} else {
			// Set as value: City
			field.Set(target)
		}
	}

	return nil
}
