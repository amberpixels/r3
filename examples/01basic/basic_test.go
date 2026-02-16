package ex1basic_test

import (
	"encoding/json"
	"fmt"

	r3json "github.com/amberpixels/r3/dialects/json"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

func ExampleJSONFilters() {
	jsonRaw := `[{"f":"Country.name","op":"eq","v":"United States"}, {"f":"popularity","op":"gt","v":50}]`

	var jsonFilters r3json.JSONFilters
	err := json.Unmarshal([]byte(jsonRaw), &jsonFilters)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fmt.Println(jsonFilters)

	filters, err := r3json.JSONToFilters(jsonFilters)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fmt.Println(filters)

	clauses, err := r3sql.FiltersToSQL(filters)

	fmt.Println(err)
	for i, clause := range clauses {
		fmt.Println("CLAUSE ... ", i)
		fmt.Println("   -", clause.Clause)
		fmt.Println("   -", clause.Args)
		fmt.Println("   -", clause.Joins)
	}

	// Output: [{"f":"Country.name","op":"eq","v":"United States"},{"f":"popularity","op":"gt","v":50}]
	// [{"Field":"Country.name","Operator":1,"Value":"United States","And":[],"Or":[]} {"Field":"popularity","Operator":4,"Value":50,"And":[],"Or":[]}]
	// <nil>
	// CLAUSE ...  0
	//    - "Country"."name" = ?
	//    - [United States]
	//    - ["Country"]
	// CLAUSE ...  1
	//    - "popularity" > ?
	//    - [50]
	//    - []
}
