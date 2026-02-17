module github.com/amberpixels/r3/engine/file

go 1.25

require (
	github.com/amberpixels/k1 v0.1.4
	github.com/amberpixels/r3 v0.0.0
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/d3rty/json v0.0.0-20260213115610-d08e6c7f6a6f // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
)

replace github.com/amberpixels/r3 => ../..
