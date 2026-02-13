module github.com/amberpixels/r3/drivers/sqlite3

go 1.25

require (
	github.com/amberpixels/r3 v0.0.0
	github.com/amberpixels/r3/sqlbase v0.0.0
	github.com/mattn/go-sqlite3 v1.14.34
	github.com/pressly/goose v2.7.0+incompatible
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/amberpixels/k1 v0.1.4 // indirect
	github.com/amberpixels/r3/dialects/sql v0.0.0 // indirect
	github.com/d3rty/json v0.0.0-20260213115610-d08e6c7f6a6f // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/amberpixels/r3 => ../..
	github.com/amberpixels/r3/dialects/sql => ../../dialects/sql
	github.com/amberpixels/r3/sqlbase => ../../sqlbase
)
