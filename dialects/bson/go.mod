module github.com/amberpixels/r3/dialects/bson

go 1.26

replace github.com/amberpixels/r3 => ../..

require (
	github.com/amberpixels/r3 v0.0.0-00010101000000-000000000000
	go.mongodb.org/mongo-driver/v2 v2.5.0
)

require github.com/amberpixels/k1 v0.1.4 // indirect
