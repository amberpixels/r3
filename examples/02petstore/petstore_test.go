package petstore_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	petstore "github.com/amberpixels/r3/examples/02petstore"
	dockerclient "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// isDockerAvailable checks if Docker (or OrbStack) is reachable.
func isDockerAvailable() bool {
	defer func() { recover() }()

	if os.Getenv("DOCKER_HOST") == "" {
		orbstackSocket := "unix:///Users/" + os.Getenv("USER") + "/.orbstack/run/docker.sock"
		os.Setenv("DOCKER_HOST", orbstackSocket)
	}

	ctx := context.Background()
	dc, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return false
	}
	defer dc.Close()

	_, err = dc.Ping(ctx, dockerclient.PingOptions{})
	return err == nil
}

// setupPostgresContainer starts a PostgreSQL container and returns a connected gorm.DB.
func setupPostgresContainer(t *testing.T) (testcontainers.Container, *gorm.DB) {
	t.Helper()

	ctx := t.Context()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "petstore",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start postgres container")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=petstore sslmode=disable", host, port.Port())
	slog.Info("PostgreSQL DSN", "dsn", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	// Auto-migrate tables for the example
	err = db.AutoMigrate(&petstore.Species{}, &petstore.Pet{})
	require.NoError(t, err, "failed to migrate models")

	return container, db
}

// seedData inserts test species and pets.
func seedData(t *testing.T, db *gorm.DB) {
	t.Helper()

	species := []petstore.Species{
		{Name: "Dog"},
		{Name: "Cat"},
		{Name: "Bird"},
		{Name: "Fish"},
	}
	for i := range species {
		require.NoError(t, db.Create(&species[i]).Error)
	}

	pets := []petstore.Pet{
		{Name: "Buddy", SpeciesID: species[0].ID, Status: "available", Age: 3, Price: 500, Tags: "friendly,trained"},
		{Name: "Max", SpeciesID: species[0].ID, Status: "available", Age: 1, Price: 800, Tags: "puppy,energetic"},
		{Name: "Whiskers", SpeciesID: species[1].ID, Status: "sold", Age: 5, Price: 200, Tags: "calm,indoor"},
		{Name: "Luna", SpeciesID: species[1].ID, Status: "available", Age: 2, Price: 350, Tags: "playful"},
		{Name: "Tweety", SpeciesID: species[2].ID, Status: "pending", Age: 1, Price: 150, Tags: "singing"},
		{Name: "Nemo", SpeciesID: species[3].ID, Status: "available", Age: 1, Price: 50, Tags: "colorful"},
	}
	for i := range pets {
		require.NoError(t, db.Create(&pets[i]).Error)
	}
}

func TestPetStoreAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	container, db := setupPostgresContainer(t)
	defer func() { _ = container.Terminate(t.Context()) }()

	seedData(t, db)

	srv := petstore.NewServer(db)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := ts.Client()

	// ---------------------------------------------------
	// GET /species - list all species
	// ---------------------------------------------------
	t.Run("list species", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/species")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data  []petstore.Species `json:"data"`
			Total int64              `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, int64(4), body.Total)
		assert.Len(t, body.Data, 4)
	})

	// ---------------------------------------------------
	// GET /pets - list all pets (default: sorted by created_at desc, page_size=20)
	// ---------------------------------------------------
	t.Run("list all pets", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/pets")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data  []petstore.Pet `json:"data"`
			Total int64          `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, int64(6), body.Total)
		assert.Len(t, body.Data, 6)

		// Verify species is preloaded
		for _, pet := range body.Data {
			assert.NotNil(t, pet.Species, "species should be preloaded for pet %s", pet.Name)
		}
	})

	// ---------------------------------------------------
	// GET /pets?filters=... - filter by status
	// ---------------------------------------------------
	t.Run("filter pets by status", func(t *testing.T) {
		filters := `[{"f":"status","op":"eq","v":"available"}]`
		resp, err := client.Get(ts.URL + "/pets?filters=" + filters)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data  []petstore.Pet `json:"data"`
			Total int64          `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, int64(4), body.Total)

		for _, pet := range body.Data {
			assert.Equal(t, "available", pet.Status)
		}
	})

	// ---------------------------------------------------
	// GET /pets?filters=... - filter by price range
	// ---------------------------------------------------
	t.Run("filter pets by price range", func(t *testing.T) {
		filters := `[{"f":"price","op":"gte","v":200},{"f":"price","op":"lte","v":500}]`
		resp, err := client.Get(ts.URL + "/pets?filters=" + filters)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data  []petstore.Pet `json:"data"`
			Total int64          `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

		for _, pet := range body.Data {
			assert.GreaterOrEqual(t, pet.Price, 200.0)
			assert.LessOrEqual(t, pet.Price, 500.0)
		}
	})

	// ---------------------------------------------------
	// GET /pets?sort=... - sort by price ascending
	// ---------------------------------------------------
	t.Run("sort pets by price asc", func(t *testing.T) {
		sort := `[{"field":"price","direction":"asc"}]`
		resp, err := client.Get(ts.URL + "/pets?sort=" + sort)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data []petstore.Pet `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.NotEmpty(t, body.Data)

		for i := 1; i < len(body.Data); i++ {
			assert.LessOrEqual(t, body.Data[i-1].Price, body.Data[i].Price,
				"pets should be sorted by price ascending")
		}
	})

	// ---------------------------------------------------
	// GET /pets?page=1&per_page=2 - pagination
	// ---------------------------------------------------
	t.Run("paginate pets", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/pets?page=1&per_page=2")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data  []petstore.Pet `json:"data"`
			Total int64          `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, int64(6), body.Total, "total should reflect all pets")
		assert.Len(t, body.Data, 2, "should return only 2 pets per page")
	})

	// ---------------------------------------------------
	// GET /pets/{id} - get a single pet
	// ---------------------------------------------------
	t.Run("get pet by id", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/pets/1")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var pet petstore.Pet
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&pet))
		assert.Equal(t, int64(1), pet.ID)
		assert.NotNil(t, pet.Species)
	})

	// ---------------------------------------------------
	// POST /pets - create a new pet
	// ---------------------------------------------------
	var createdPetID int64
	t.Run("create pet", func(t *testing.T) {
		body := map[string]any{
			"name":       "Charlie",
			"species_id": 1,
			"status":     "available",
			"age":        2,
			"price":      600,
			"tags":       "gentle,big",
		}
		b, _ := json.Marshal(body)

		resp, err := client.Post(ts.URL+"/pets", "application/json", bytes.NewReader(b))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var pet petstore.Pet
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&pet))
		assert.Equal(t, "Charlie", pet.Name)
		assert.NotZero(t, pet.ID)
		createdPetID = pet.ID
	})

	// ---------------------------------------------------
	// PUT /pets/{id} - full update
	// ---------------------------------------------------
	t.Run("full update pet", func(t *testing.T) {
		body := map[string]any{
			"name":       "Charlie Updated",
			"species_id": 1,
			"status":     "pending",
			"age":        3,
			"price":      650,
			"tags":       "gentle,big,trained",
		}
		b, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/pets/%d", ts.URL, createdPetID), bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var pet petstore.Pet
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&pet))
		assert.Equal(t, "Charlie Updated", pet.Name)
		assert.Equal(t, "pending", pet.Status)
		assert.InDelta(t, 650.0, pet.Price, 0.01)
	})

	// ---------------------------------------------------
	// PATCH /pets/{id} - partial update (only change status)
	// ---------------------------------------------------
	t.Run("patch pet status", func(t *testing.T) {
		body := map[string]any{
			"status": "sold",
		}
		b, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/pets/%d", ts.URL, createdPetID), bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var pet petstore.Pet
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&pet))
		assert.Equal(t, "sold", pet.Status)
		assert.Equal(t, "Charlie Updated", pet.Name, "name should be preserved from previous update")
	})

	// ---------------------------------------------------
	// DELETE /pets/{id} - soft delete
	// ---------------------------------------------------
	t.Run("delete pet", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/pets/%d", ts.URL, createdPetID), nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify it's gone from regular listing
		resp2, err := client.Get(fmt.Sprintf("%s/pets/%d", ts.URL, createdPetID))
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
	})

	// ---------------------------------------------------
	// POST /species - create a new species
	// ---------------------------------------------------
	t.Run("create species", func(t *testing.T) {
		body := map[string]any{"name": "Hamster"}
		b, _ := json.Marshal(body)

		resp, err := client.Post(ts.URL+"/species", "application/json", bytes.NewReader(b))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var sp petstore.Species
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&sp))
		assert.Equal(t, "Hamster", sp.Name)
		assert.NotZero(t, sp.ID)
	})

	// ---------------------------------------------------
	// Combined: filter + sort + pagination
	// ---------------------------------------------------
	t.Run("filter sort paginate combined", func(t *testing.T) {
		filters := `[{"f":"status","op":"eq","v":"available"}]`
		sort := `[{"field":"price","direction":"desc"}]`
		url := fmt.Sprintf("%s/pets?filters=%s&sort=%s&page=1&per_page=2", ts.URL, filters, sort)

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data  []petstore.Pet `json:"data"`
			Total int64          `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, int64(4), body.Total, "4 available pets total")
		assert.Len(t, body.Data, 2, "page size = 2")

		// Should be desc by price
		if len(body.Data) == 2 {
			assert.GreaterOrEqual(t, body.Data[0].Price, body.Data[1].Price)
		}
	})
}
