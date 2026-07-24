package r3_test

import (
	"testing"
	"time"

	"github.com/expectto/be"

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
	be.RequireThat(t, ok, be.True())
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
			be.AssertThat(t, a.Caps, be.Eq(tc.caps))
			be.AssertThat(t, a.Type, be.Eq(tc.typ))
		})
	}
}

func TestSchemaOf_SecretIsHidden(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	a := attr(t, s, "secret_token")
	for _, c := range []r3.Capability{r3.Filterable, r3.Sortable, r3.Queryable} {
		be.AssertThat(t, a.Has(c), be.False())
	}
}

func TestSchemaOf_EnumValues(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	a := attr(t, s, "status")
	be.AssertThat(t, a.Enum, be.Eq([]string{"draft", "planned", "published"}))
}

func TestSchemaOf_OperatorsByType(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()

	// Numeric attributes get range operators; strings get LIKE family.
	be.AssertThat(t, attr(t, s, "weight").Ops, be.ContainElement(r3.OperatorBetween))
	be.AssertThat(t, attr(t, s, "title").Ops, be.Not(be.ContainElement(r3.OperatorBetween)))
	be.AssertThat(t, attr(t, s, "title").Ops, be.ContainElement(r3.OperatorILike))
	// Non-filterable attribute carries no operators.
	be.AssertThat(t, attr(t, s, "secret_token").Ops, be.Nil())
}

func TestSchemaOf_Relation(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	a := attr(t, s, "children")
	be.RequireThat(t, a.Type, be.Eq(r3.TypeRel))
	be.AssertThat(t, a.Caps, be.Eq(r3.Queryable))
	be.RequireThat(t, a.Relation, be.NotNil())
	be.AssertThat(t, a.Relation.Target, be.Eq("schema_child"))
	be.AssertThat(t, a.Relation.Kind, be.Eq("has-many"))
}

func TestSchemaOf_SkipsIgnoredAndUnexported(t *testing.T) {
	s := r3.SchemaOf[schemaModel]()
	_, ok := s.Lookup("ignored")
	be.AssertThat(t, ok, be.False())
}

func TestSchemaOf_DeterministicOrder(t *testing.T) {
	a := r3.SchemaOf[schemaModel]().Attributes()
	b := r3.SchemaOf[schemaModel]().Attributes()
	be.RequireThat(t, a, be.HaveLength(len(b)))
	for i := range a {
		be.AssertThat(t, a[i].Name, be.Eq(b[i].Name))
	}
	// First declared attribute must come first.
	be.RequireThat(t, a, be.NotEmpty())
	be.AssertThat(t, a[0].Name, be.Eq("id"))
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
		be.AssertThat(t, s.Writable(tc.name, r3.WriteOpCreate), be.Eq(tc.create))
		be.AssertThat(t, s.Writable(tc.name, r3.WriteOpMutate), be.Eq(tc.mutate))
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
	be.AssertThat(t, s.Writable("title", r3.WriteOpMutate), be.True())
	be.AssertThat(t, attr(t, s2, "title").Has(r3.Mutable), be.False())
	c := attr(t, s2, "raids_created")
	be.AssertThat(t, c.Computed, be.True())
	be.AssertThat(t, c.Has(r3.Sortable), be.True())
}
