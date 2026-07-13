package r3schema_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3schema "github.com/amberpixels/r3/dialects/schema"
	"github.com/expectto/be"
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
	be.NoError(t, err)

	var indented bytes.Buffer
	err = json.Indent(&indented, data, "", "  ")
	be.NoError(t, err)
	indented.WriteByte('\n')

	want, err := os.ReadFile("testdata/campaign.json")
	be.NoError(t, err)

	be.AssertThat(t, indented.Bytes(), be.Eq(want))
}

func TestMarshalSchema_OmitsNonQueryable(t *testing.T) {
	data, err := r3schema.MarshalSchema(r3.SchemaOf[Campaign]())
	be.NoError(t, err)

	be.AssertThat(t, bytes.Contains(data, []byte("secret_token")), be.False())
}

func TestMarshalSchema_OperatorsMatchTypeDefaults(t *testing.T) {
	data, err := r3schema.MarshalSchema(r3.SchemaOf[Campaign]())
	be.NoError(t, err)

	var decoded struct {
		Version    int `json:"version"`
		Attributes []struct {
			Name string   `json:"name"`
			Type string   `json:"type"`
			Ops  []string `json:"ops"`
			Enum []string `json:"enum"`
		} `json:"attributes"`
	}
	err = json.Unmarshal(data, &decoded)
	be.NoError(t, err)
	be.AssertThat(t, decoded.Version, be.Eq(r3schema.Version))

	ops := map[string][]string{}
	for _, a := range decoded.Attributes {
		ops[a.Name] = a.Ops
	}
	// A numeric column carries range operators; a string column carries LIKE.
	be.AssertThat(t, ops["budget"], be.ContainElement("between"))
	be.AssertThat(t, ops["title"], be.Not(be.ContainElement("between")))
	be.AssertThat(t, ops["title"], be.ContainElement("ilike"))
}
