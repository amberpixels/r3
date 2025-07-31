package ex1basic_test

import (
	"encoding/json"
	"fmt"

	r3json "github.com/amberpixels/r3/dialects/json"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

func ExampleJSONFilters() {
	jsonRaw := `[{"f":"\"Country\".name","op":"eq","v":"United States"}, {"f":"popularity","op":"gt","v":50}]`

	var inboundFilters r3json.JSONFilters
	err := json.Unmarshal([]byte(jsonRaw), &inboundFilters)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fmt.Println(inboundFilters)

	filters, err := r3json.JSONFiltersToFilters(inboundFilters)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fmt.Println(filters)

	clauses, err := r3sql.FiltersToSQLClauses(filters)

	fmt.Println(err)
	for i, clause := range clauses {
		fmt.Println("CLAUSE ... ", i)
		fmt.Println("   -", clause.Clause)
		fmt.Println("   -", clause.Args)
		fmt.Println("   -", clause.Joins)
	}

	//Output: [{"JSONField":"country_name","Operator":1,"Value":"United States","And":[],"Or":[]} {"JSONField":"popularity","Operator":4,"Value":50,"And":[],"Or":[]}]
}
