package r3_test

import (
	"slices"
	"testing"
	"time"

	"github.com/amberpixels/r3"
)

// schemaModel exercises every tag combination and default-rule exception.
type schemaModel struct {
	ID        int64     `r3:"id,pk"`                                    // read-only identity
	Title     string    `r3:"title"`                                    // all caps
	Weight    int       `r3:"weight"`                                   // numeric scalar
	Ratio     float64   `r3:"ratio"`                                    // float scalar
	Active    bool      `r3:"active"`                                   // bool scalar
	Status    string    `r3:"status,enum:draft|planned|published"`      // enum
	Slug      string    `r3:"slug,immutable"`                           // set-once
	Secret    string    `r3:"secret_token,no-filter,no-sort,no-output"` // fully hidden
	Pop       int       `r3:"population,readonly"`                      // feed-synced, read-only
	CreatedAt time.Time `r3:"created_at"`                               // server timestamp
	UpdatedAt time.Time `r3:"updated_at"`                               // system timestamp
	DeletedAt time.Time `r3:"deleted_at,soft_delete"`                   // soft-delete
	Ignored   string    `r3:"-"`                                        // skipped

	Children []schemaChild `r3:"rel:has-many,fk:parent_id"` // relation
}

type schemaChild struct {
	ID       int64 `r3:"id,pk"`
	ParentID int64 `r3:"parent_id"`
	Name     string
}

func attr(t *testing.T, s r3.Schema, name string) r3.Attribute {
	t.Helper()
	a, ok := s.Lookup(name)
	if !ok {
		t.Fatalf("attribute %q not found in schema", name)
	}
	return a
}

func TestSchemaOf_DefaultCapabilities(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()

	cases := []struct {
		name string
		caps r3.Capability
		typ  r3.DataType
	}{
		// Plain scalar: everything.
		{"title", r3.Filterable | r3.Sortable | r3.Queryable | r3.Creatable | r3.Mutable, r3.TypeString},
		{"weight", r3.Filterable | r3.Sortable | r3.Queryable | r3.Creatable | r3.Mutable, r3.TypeInt},
		{"ratio", r3.Filterable | r3.Sortable | r3.Queryable | r3.Creatable | r3.Mutable, r3.TypeFloat},
		{"active", r3.Filterable | r3.Sortable | r3.Queryable | r3.Creatable | r3.Mutable, r3.TypeBool},
		{"status", r3.Filterable | r3.Sortable | r3.Queryable | r3.Creatable | r3.Mutable, r3.TypeEnum},
		// PK: read-only identity, still queryable/filterable/sortable.
		{"id", r3.Filterable | r3.Sortable | r3.Queryable, r3.TypeInt},
		// immutable: creatable once, not mutable.
		{"slug", r3.Filterable | r3.Sortable | r3.Queryable | r3.Creatable, r3.TypeString},
		// readonly feed column: not creatable, not mutable.
		{"population", r3.Filterable | r3.Sortable | r3.Queryable, r3.TypeInt},
		// fully hidden secret.
		{"secret_token", r3.Creatable | r3.Mutable, r3.TypeString},
		// timestamps and soft-delete: read-only.
		{"created_at", r3.Filterable | r3.Sortable | r3.Queryable, r3.TypeTime},
		{"updated_at", r3.Filterable | r3.Sortable | r3.Queryable, r3.TypeTime},
		{"deleted_at", r3.Filterable | r3.Sortable | r3.Queryable, r3.TypeTime},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := attr(t, s, tc.name)
			if a.Caps != tc.caps {
				t.Errorf("caps = %05b, want %05b", a.Caps, tc.caps)
			}
			if a.Type != tc.typ {
				t.Errorf("type = %q, want %q", a.Type, tc.typ)
			}
		})
	}
}

func TestSchemaOf_SecretIsHidden(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	a := attr(t, s, "secret_token")
	for _, c := range []r3.Capability{r3.Filterable, r3.Sortable, r3.Queryable} {
		if a.Has(c) {
			t.Errorf("secret_token should not have capability %05b", c)
		}
	}
}

func TestSchemaOf_EnumValues(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	a := attr(t, s, "status")
	want := []string{"draft", "planned", "published"}
	if !slices.Equal(a.Enum, want) {
		t.Errorf("enum = %v, want %v", a.Enum, want)
	}
}

func TestSchemaOf_OperatorsByType(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()

	// Numeric attributes get range operators; strings get LIKE family.
	if !slices.Contains(attr(t, s, "weight").Ops, r3.OperatorBetween) {
		t.Error("weight (int) should allow Between")
	}
	if slices.Contains(attr(t, s, "title").Ops, r3.OperatorBetween) {
		t.Error("title (string) should not allow Between")
	}
	if !slices.Contains(attr(t, s, "title").Ops, r3.OperatorILike) {
		t.Error("title (string) should allow ILike")
	}
	// Non-filterable attribute carries no operators.
	if ops := attr(t, s, "secret_token").Ops; ops != nil {
		t.Errorf("secret_token should have no operators, got %v", ops)
	}
}

func TestSchemaOf_Relation(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	a := attr(t, s, "children")
	if a.Type != r3.TypeRel {
		t.Fatalf("children type = %q, want relation", a.Type)
	}
	if a.Caps != r3.Queryable {
		t.Errorf("relation caps = %05b, want queryable-only", a.Caps)
	}
	if a.Relation == nil || a.Relation.Target != "schema_child" || a.Relation.Kind != "has-many" {
		t.Errorf("relation ref = %+v, want target=schema_child kind=has-many", a.Relation)
	}
}

func TestSchemaOf_SkipsIgnoredAndUnexported(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	if _, ok := s.Lookup("ignored"); ok {
		t.Error("r3:\"-\" field should be skipped")
	}
}

func TestSchemaOf_DeterministicOrder(t *testing.T) {
	a := r3.SchemaOf[schemaModel]().Attributes()
	b := r3.SchemaOf[schemaModel]().Attributes()
	if len(a) != len(b) {
		t.Fatalf("attribute count changed between calls: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Errorf("order differs at %d: %q vs %q", i, a[i].Name, b[i].Name)
		}
	}
	// First declared attribute must come first.
	if len(a) == 0 || a[0].Name != "id" {
		t.Errorf("first attribute = %q, want id", a[0].Name)
	}
}

func TestSchemaOf_Writable(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	cases := []struct {
		name           string
		create, mutate bool
	}{
		{"title", true, true},
		{"slug", true, false},        // immutable
		{"population", false, false}, // readonly
		{"id", false, false},         // pk
		{"created_at", false, false},
		{"missing", false, false},
	}
	for _, tc := range cases {
		if got := s.Writable(tc.name, r3.WriteOpCreate); got != tc.create {
			t.Errorf("Writable(%q, create) = %v, want %v", tc.name, got, tc.create)
		}
		if got := s.Writable(tc.name, r3.WriteOpMutate); got != tc.mutate {
			t.Errorf("Writable(%q, mutate) = %v, want %v", tc.name, got, tc.mutate)
		}
	}
}

func TestSchemaOf_With_Override(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	// Override an existing attribute and add a computed one.
	s2 := s.With(
		r3.Attribute{Name: "title", Type: r3.TypeString, Caps: r3.Queryable},
		r3.Attribute{Name: "raids_created", Type: r3.TypeInt, Caps: r3.Queryable | r3.Sortable, Computed: true},
	)

	// Original schema is unchanged (immutability).
	if !s.Writable("title", r3.WriteOpMutate) {
		t.Error("original schema mutated by With")
	}
	if a := attr(t, s2, "title"); a.Has(r3.Mutable) {
		t.Error("override did not tighten title caps")
	}
	c := attr(t, s2, "raids_created")
	if !c.Computed || !c.Has(r3.Sortable) {
		t.Errorf("computed attribute not added correctly: %+v", c)
	}
}
