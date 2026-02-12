package petstore

import (
	"net/http"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"gorm.io/gorm"
)

// Server is the pet store HTTP server.
type Server struct {
	petRepo     r3.CRUD[Pet, int64]
	speciesRepo r3.CRUD[Species, int64]
	mux         *http.ServeMux
}

// NewServer creates a new Server backed by the given gorm.DB.
func NewServer(db *gorm.DB) *Server {
	petRepo := r3gorm.NewGormCRUD[Pet, int64](db)
	petRepo.SetDefaultListQuery(r3.Query{
		Sorts:      r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))},
		Pagination: r3.NewPaginationSpecWithSize(20),
	})

	speciesRepo := r3gorm.NewGormCRUD[Species, int64](db)

	s := &Server{
		petRepo:     petRepo,
		speciesRepo: speciesRepo,
		mux:         http.NewServeMux(),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Swagger UI
	s.mux.HandleFunc("GET /", s.handleSwaggerUI)
	s.mux.HandleFunc("GET /openapi.json", s.handleOpenAPISpec)

	// Pets
	s.mux.HandleFunc("GET /pets", s.handleListPets)
	s.mux.HandleFunc("GET /pets/{id}", s.handleGetPet)
	s.mux.HandleFunc("POST /pets", s.handleCreatePet)
	s.mux.HandleFunc("PUT /pets/{id}", s.handleUpdatePet)
	s.mux.HandleFunc("PATCH /pets/{id}", s.handlePatchPet)
	s.mux.HandleFunc("DELETE /pets/{id}", s.handleDeletePet)

	// Species
	s.mux.HandleFunc("GET /species", s.handleListSpecies)
	s.mux.HandleFunc("POST /species", s.handleCreateSpecies)
}
