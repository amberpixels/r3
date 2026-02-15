module github.com/amberpixels/r3/r3history/stores/gorm

go 1.25

require (
	github.com/amberpixels/r3 v0.0.0
	github.com/amberpixels/r3/r3history v0.0.0
	gorm.io/gorm v1.31.1
)

require (
	github.com/amberpixels/k1 v0.1.4 // indirect
	github.com/d3rty/json v0.0.0-20260213115610-d08e6c7f6a6f // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/text v0.30.0 // indirect
)

replace (
	github.com/amberpixels/r3 => ../../..
	github.com/amberpixels/r3/r3history => ../..
)
