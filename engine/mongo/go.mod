module github.com/amberpixels/r3/engine/mongo

go 1.25

require (
	github.com/amberpixels/r3 v0.0.0
	github.com/amberpixels/r3/dialects/bson v0.0.0
	go.mongodb.org/mongo-driver/v2 v2.5.0
)

require (
	github.com/amberpixels/k1 v0.1.4 // indirect
	github.com/d3rty/json v0.0.0-20260213115610-d08e6c7f6a6f // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)

replace (
	github.com/amberpixels/r3 => ../..
	github.com/amberpixels/r3/dialects/bson => ../../dialects/bson
)
