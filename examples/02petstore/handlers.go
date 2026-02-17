package petstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// --- Pets ---

// handleListPets handles GET /pets.
//
// Query parameters:
//
//	filters  - JSON array of r3 filter objects, e.g. [{"f":"status","op":"eq","v":"available"}]
//	sort     - JSON array of r3 sort objects, e.g. [{"field":"price","direction":"asc"}]
//	page     - page number (default 1)
//	per_page - page size   (default 20)
func (s *Server) handleListPets(w http.ResponseWriter, r *http.Request) {
	q, err := queryFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Always preload species
	q.Preloads = r3.Preloads{r3.NewPreloadSpec("Species")}

	pets, total, err := s.petRepo.List(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  pets,
		"total": total,
	})
}

// handleGetPet handles GET /pets/{id}.
func (s *Server) handleGetPet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	pet, err := s.petRepo.Get(r.Context(), id, r3.Query{
		Preloads: r3.Preloads{r3.NewPreloadSpec("Species")},
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "pet not found")
		return
	}

	writeJSON(w, http.StatusOK, pet)
}

// handleCreatePet handles POST /pets.
func (s *Server) handleCreatePet(w http.ResponseWriter, r *http.Request) {
	var pet Pet
	if err := json.NewDecoder(r.Body).Decode(&pet); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Reset server-controlled fields.
	pet.ID = 0

	created, err := s.petRepo.Create(r.Context(), pet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// handleUpdatePet handles PUT /pets/{id} (full replace).
func (s *Server) handleUpdatePet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var pet Pet
	if err := json.NewDecoder(r.Body).Decode(&pet); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pet.ID = id

	updated, err := s.petRepo.Update(r.Context(), pet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// handlePatchPet handles PATCH /pets/{id} (partial update).
func (s *Server) handlePatchPet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	// Get current pet.
	existing, err := s.petRepo.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "pet not found")
		return
	}

	// Decode patch on top of existing.
	if err := json.NewDecoder(r.Body).Decode(&existing); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	existing.ID = id // ensure ID is not overwritten

	updated, err := s.petRepo.Update(r.Context(), existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// handleDeletePet handles DELETE /pets/{id}.
func (s *Server) handleDeletePet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.petRepo.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Species ---

// handleListSpecies handles GET /species.
func (s *Server) handleListSpecies(w http.ResponseWriter, r *http.Request) {
	species, total, err := s.speciesRepo.List(r.Context(), r3.Query{
		Pagination: r3.NoPagination(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  species,
		"total": total,
	})
}

// handleCreateSpecies handles POST /species.
func (s *Server) handleCreateSpecies(w http.ResponseWriter, r *http.Request) {
	var sp Species
	if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sp.ID = 0

	created, err := s.speciesRepo.Create(r.Context(), sp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// --- Helpers ---

// queryFromRequest parses r3 query parameters (filters, sort, page, per_page) from the HTTP request.
func queryFromRequest(r *http.Request) (r3.Query, error) {
	q := r3.NewQuery()

	// Parse filters
	if raw := r.URL.Query().Get("filters"); raw != "" {
		var jFilters r3json.JSONFilters
		if err := json.Unmarshal([]byte(raw), &jFilters); err != nil {
			return q, fmt.Errorf("invalid filters: %w", err)
		}
		filters, err := r3json.JSONToFilters(jFilters)
		if err != nil {
			return q, fmt.Errorf("invalid filters: %w", err)
		}
		q.Filters = filters
	}

	// Parse sort
	if raw := r.URL.Query().Get("sort"); raw != "" {
		var jSorts []r3json.JSONSort
		if err := json.Unmarshal([]byte(raw), &jSorts); err != nil {
			return q, fmt.Errorf("invalid sort: %w", err)
		}
		for _, js := range jSorts {
			s, err := r3json.JSONToSort(&js)
			if err != nil {
				return q, fmt.Errorf("invalid sort: %w", err)
			}
			q.Sorts = append(q.Sorts, s)
		}
	}

	// Parse pagination
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return q, errors.New("invalid page number")
		}
		perPage := 20
		if ppStr := r.URL.Query().Get("per_page"); ppStr != "" {
			perPage, err = strconv.Atoi(ppStr)
			if err != nil || perPage < 1 {
				return q, errors.New("invalid per_page value")
			}
		}
		q.Pagination = r3.NewPaginationSpec(page, perPage)
	}

	return q, nil
}

// pathID extracts the {id} path parameter as int64.
func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
