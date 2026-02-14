module github.com/amberpixels/r3/sqlbase

go 1.25

require (
	github.com/amberpixels/r3 v0.0.0
	github.com/amberpixels/r3/dialects/sql v0.0.0
)

require (
	github.com/amberpixels/k1 v0.1.4 // indirect
	github.com/d3rty/json v0.0.0-20260213115610-d08e6c7f6a6f // indirect
)

replace (
	github.com/amberpixels/r3 => ..
	github.com/amberpixels/r3/dialects/sql => ../dialects/sql
)
