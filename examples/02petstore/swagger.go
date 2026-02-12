package petstore

import "net/http"

// handleSwaggerUI serves the Swagger UI page with an inline OpenAPI 3.0 spec.
func (s *Server) handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerHTML)) //nolint: errcheck
}

// handleOpenAPISpec serves the raw OpenAPI JSON spec.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(openAPISpec)) //nolint: errcheck
}

const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "r3 Pet Store",
    "description": "Example CRUD API built with the r3 library (github.com/amberpixels/r3).\n\nDemonstrates filtering, sorting, and pagination via r3's JSON dialect.",
    "version": "1.0.0"
  },
  "servers": [{"url": "/"}],
  "tags": [
    {"name": "Pets", "description": "Pet CRUD operations"},
    {"name": "Species", "description": "Species reference data"}
  ],
  "paths": {
    "/pets": {
      "get": {
        "tags": ["Pets"],
        "summary": "List pets",
        "description": "Returns a paginated list of pets. Supports r3 filtering, sorting, and pagination via query parameters.",
        "parameters": [
          {
            "name": "filters",
            "in": "query",
            "description": "JSON array of r3 filter objects.\n\nExamples:\n- By status: [  {\"f\":\"status\",\"op\":\"eq\",\"v\":\"available\"}]\n- By price range: [{\"f\":\"price\",\"op\":\"gte\",\"v\":200},{\"f\":\"price\",\"op\":\"lte\",\"v\":500}]\n- By species: [{\"f\":\"species_id\",\"op\":\"eq\",\"v\":1}]\n- ILIKE search: [{\"f\":\"name\",\"op\":\"ilike\",\"v\":\"%buddy%\"}]\n\nSupported operators: eq, ne, gt, gte, lt, lte, in, nin, like, notlike, ilike, exists, between",
            "schema": {"type": "string"},
            "example": "[{\"f\":\"status\",\"op\":\"eq\",\"v\":\"available\"}]"
          },
          {
            "name": "sort",
            "in": "query",
            "description": "JSON array of r3 sort objects.\n\nExample: [{\"field\":\"price\",\"direction\":\"asc\"}]\n\nDirections: asc, desc",
            "schema": {"type": "string"},
            "example": "[{\"field\":\"price\",\"direction\":\"asc\"}]"
          },
          {
            "name": "page",
            "in": "query",
            "description": "Page number (1-indexed)",
            "schema": {"type": "integer", "default": 1}
          },
          {
            "name": "per_page",
            "in": "query",
            "description": "Items per page",
            "schema": {"type": "integer", "default": 20}
          }
        ],
        "responses": {
          "200": {
            "description": "Paginated list of pets",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "data": {"type": "array", "items": {"$ref": "#/components/schemas/Pet"}},
                    "total": {"type": "integer", "description": "Total matching pets (before pagination)"}
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "tags": ["Pets"],
        "summary": "Create a pet",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/PetInput"},
              "example": {"name": "Buddy", "species_id": 1, "status": "available", "age": 3, "price": 500, "tags": "friendly,trained"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Created pet",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Pet"}}}
          }
        }
      }
    },
    "/pets/{id}": {
      "get": {
        "tags": ["Pets"],
        "summary": "Get a pet by ID",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {
          "200": {
            "description": "Pet details",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Pet"}}}
          },
          "404": {"description": "Pet not found"}
        }
      },
      "put": {
        "tags": ["Pets"],
        "summary": "Full update a pet",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/PetInput"},
              "example": {"name": "Buddy Updated", "species_id": 1, "status": "sold", "age": 4, "price": 450, "tags": "friendly,trained,senior"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Updated pet",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Pet"}}}
          }
        }
      },
      "patch": {
        "tags": ["Pets"],
        "summary": "Partial update a pet",
        "description": "Only the provided fields are updated. Omitted fields keep their current values.",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/PetPatch"},
              "example": {"status": "sold"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Updated pet",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Pet"}}}
          },
          "404": {"description": "Pet not found"}
        }
      },
      "delete": {
        "tags": ["Pets"],
        "summary": "Delete a pet (soft delete)",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {
          "204": {"description": "Pet deleted"}
        }
      }
    },
    "/species": {
      "get": {
        "tags": ["Species"],
        "summary": "List all species",
        "responses": {
          "200": {
            "description": "All species",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "data": {"type": "array", "items": {"$ref": "#/components/schemas/Species"}},
                    "total": {"type": "integer"}
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "tags": ["Species"],
        "summary": "Create a species",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/SpeciesInput"},
              "example": {"name": "Turtle"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Created species",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Species"}}}
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Pet": {
        "type": "object",
        "properties": {
          "id":         {"type": "integer"},
          "name":       {"type": "string"},
          "species_id": {"type": "integer"},
          "species":    {"$ref": "#/components/schemas/Species"},
          "status":     {"type": "string", "enum": ["available", "pending", "sold"]},
          "age":        {"type": "integer"},
          "price":      {"type": "number"},
          "tags":       {"type": "string", "description": "Comma-separated tags"},
          "created_at": {"type": "string", "format": "date-time"},
          "updated_at": {"type": "string", "format": "date-time"}
        }
      },
      "PetInput": {
        "type": "object",
        "required": ["name", "species_id"],
        "properties": {
          "name":       {"type": "string"},
          "species_id": {"type": "integer"},
          "status":     {"type": "string", "enum": ["available", "pending", "sold"], "default": "available"},
          "age":        {"type": "integer"},
          "price":      {"type": "number"},
          "tags":       {"type": "string"}
        }
      },
      "PetPatch": {
        "type": "object",
        "description": "All fields optional - only provided fields are updated.",
        "properties": {
          "name":       {"type": "string"},
          "species_id": {"type": "integer"},
          "status":     {"type": "string", "enum": ["available", "pending", "sold"]},
          "age":        {"type": "integer"},
          "price":      {"type": "number"},
          "tags":       {"type": "string"}
        }
      },
      "Species": {
        "type": "object",
        "properties": {
          "id":   {"type": "integer"},
          "name": {"type": "string"}
        }
      },
      "SpeciesInput": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string"}
        }
      }
    }
  }
}`

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>r3 Pet Store - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar { display: none !important; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`
