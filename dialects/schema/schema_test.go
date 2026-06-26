package r3schema_test

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3schema "github.com/amberpixels/r3/dialects/schema"
)

// Campaign is a representative model covering scalars, enum, immutable/readonly
// columns, a hidden (no-output) column, a server timestamp, and a relation.
type Campaign struct {
	ID        int64     `r3:"id,pk"`
	Title     string    `r3:"title"`
	Status    string    `r3:"status,enum:draft|active|paused"`
	Budget    float64   `r3:"budget"`
	Slug      string    `r3:"slug,immutable"`
	Spend     int       `r3:"spend,readonly"`
	Secret    string    `r3:"secret_token,no-output"`
	CreatedAt time.Time `r3:"created_at"`
	Adsets    []Adset   `r3:"rel:has-many,fk:campaign_id"`
}

type Adset struct {
	ID         int64 `r3:"id,pk"`
	CampaignID int64 `r3:"campaign_id"`
	Name       string
}

func TestMarshalSchema_Golden(t *testing.T) {
	data, err := r3schema.MarshalSchema(r3.SchemaOf[Campaign]())
	if err != nil {
		t.Fatalf("MarshalSchema: %v", err)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, data, "", "  "); err != nil {
		t.Fatalf("indent: %v", err)
	}
	indented.WriteByte('\n')

	want, err := os.ReadFile("testdata/campaign.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(indented.Bytes(), want) {
		t.Errorf("schema JSON does not match golden.\n--- got ---\n%s\n--- want ---\n%s",
			indented.String(), want)
	}
}

func TestMarshalSchema_OmitsNonQueryable(t *testing.T) {
	data, err := r3schema.MarshalSchema(r3.SchemaOf[Campaign]())
	if err != nil {
		t.Fatalf("MarshalSchema: %v", err)
	}
	if bytes.Contains(data, []byte("secret_token")) {
		t.Error("non-queryable attribute secret_token must not be serialized")
	}
}

func TestMarshalSchema_OperatorsMatchTypeDefaults(t *testing.T) {
	data, err := r3schema.MarshalSchema(r3.SchemaOf[Campaign]())
	if err != nil {
		t.Fatalf("MarshalSchema: %v", err)
	}

	var decoded struct {
		Version    int `json:"version"`
		Attributes []struct {
			Name string   `json:"name"`
			Type string   `json:"type"`
			Ops  []string `json:"ops"`
			Enum []string `json:"enum"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Version != r3schema.Version {
		t.Errorf("version = %d, want %d", decoded.Version, r3schema.Version)
	}

	ops := map[string][]string{}
	for _, a := range decoded.Attributes {
		ops[a.Name] = a.Ops
	}
	// A numeric column carries range operators; a string column carries LIKE.
	if !slices.Contains(ops["budget"], "between") {
		t.Error("budget (float) should expose between")
	}
	if slices.Contains(ops["title"], "between") {
		t.Error("title (string) should not expose between")
	}
	if !slices.Contains(ops["title"], "ilike") {
		t.Error("title (string) should expose ilike")
	}
}
