// Pet Store example server.
//
// Prerequisites: a running PostgreSQL instance.
//
// Quick start with Docker:
//
//	docker run -d --name petstore-pg -p 5432:5432 \
//	  -e POSTGRES_USER=petstore -e POSTGRES_PASSWORD=petstore -e POSTGRES_DB=petstore \
//	  postgres:18-alpine
//
// Then run:
//
//	go run ./examples/02petstore/cmd
//
// Open http://localhost:8080 for Swagger UI.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	petstore "github.com/amberpixels/r3/examples/02petstore"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := envOr("DATABASE_URL",
		"host=localhost port=5432 user=petstore password=petstore dbname=petstore sslmode=disable")
	addr := envOr("ADDR", ":8080")

	slog.Info("connecting to database", "dsn", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Auto-migrate tables.
	if err := db.AutoMigrate(&petstore.Species{}, &petstore.Pet{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// Seed data if the species table is empty.
	var count int64
	db.Model(&petstore.Species{}).Count(&count)
	if count == 0 {
		slog.Info("seeding initial data")
		seed(db)
	}

	srv := petstore.NewServer(db)

	fmt.Printf("\n  Pet Store API running on http://localhost%s\n", addr)
	fmt.Printf("  Swagger UI:  http://localhost%s/\n\n", addr)

	log.Fatal(http.ListenAndServe(addr, srv))
}

func seed(db *gorm.DB) {
	species := []petstore.Species{
		{Name: "Dog"},
		{Name: "Cat"},
		{Name: "Bird"},
		{Name: "Fish"},
		{Name: "Hamster"},
	}
	for i := range species {
		db.Create(&species[i])
	}

	pets := []petstore.Pet{
		{Name: "Buddy", SpeciesID: species[0].ID, Status: "available", Age: 3, Price: 500, Tags: "friendly,trained"},
		{Name: "Max", SpeciesID: species[0].ID, Status: "available", Age: 1, Price: 800, Tags: "puppy,energetic"},
		{Name: "Rex", SpeciesID: species[0].ID, Status: "sold", Age: 7, Price: 300, Tags: "guard,loyal"},
		{Name: "Whiskers", SpeciesID: species[1].ID, Status: "sold", Age: 5, Price: 200, Tags: "calm,indoor"},
		{Name: "Luna", SpeciesID: species[1].ID, Status: "available", Age: 2, Price: 350, Tags: "playful"},
		{Name: "Mittens", SpeciesID: species[1].ID, Status: "available", Age: 1, Price: 400, Tags: "kitten,fluffy"},
		{Name: "Tweety", SpeciesID: species[2].ID, Status: "pending", Age: 1, Price: 150, Tags: "singing"},
		{Name: "Polly", SpeciesID: species[2].ID, Status: "available", Age: 3, Price: 250, Tags: "talking,colorful"},
		{Name: "Nemo", SpeciesID: species[3].ID, Status: "available", Age: 1, Price: 50, Tags: "colorful,tropical"},
		{Name: "Goldie", SpeciesID: species[3].ID, Status: "available", Age: 2, Price: 30, Tags: "goldfish"},
		{Name: "Hammy", SpeciesID: species[4].ID, Status: "available", Age: 1, Price: 25, Tags: "small,cute"},
	}
	for i := range pets {
		db.Create(&pets[i])
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
