package r3tag_test

import (
	"reflect"
	"slices"
	"testing"

	r3tag "github.com/amberpixels/r3/internal/tag"
)

type capModel struct {
	ID     int64  `r3:"id,pk"`
	Title  string `r3:"title"`
	Slug   string `r3:"slug,immutable"`
	Pop    int    `r3:"population,readonly"`
	Secret string `r3:"secret_token,no-filter,no-sort,no-output"`
	Status string `r3:"status,enum:draft|planned|published"`
	Del    string `r3:"deleted_at,soft_delete"`
}

func fieldByName(t *testing.T, name string) reflect.StructField {
	t.Helper()
	f, ok := reflect.TypeFor[capModel]().FieldByName(name)
	if !ok {
		t.Fatalf("field %q not found", name)
	}
	return f
}

func TestParseColumnTag_CapabilityFlags(t *testing.T) {
	slug := r3tag.ParseColumnTag(fieldByName(t, "Slug"))
	if slug.Column != "slug" || !slug.Immutable || slug.ReadOnly {
		t.Errorf("slug = %+v, want column=slug immutable=true readonly=false", slug)
	}

	pop := r3tag.ParseColumnTag(fieldByName(t, "Pop"))
	if pop.Column != "population" || !pop.ReadOnly {
		t.Errorf("population = %+v, want readonly=true", pop)
	}

	secret := r3tag.ParseColumnTag(fieldByName(t, "Secret"))
	if !secret.NoFilter || !secret.NoSort || !secret.NoOutput {
		t.Errorf("secret = %+v, want all hide flags set", secret)
	}

	status := r3tag.ParseColumnTag(fieldByName(t, "Status"))
	if want := []string{"draft", "planned", "published"}; !slices.Equal(status.Enum, want) {
		t.Errorf("status enum = %v, want %v", status.Enum, want)
	}
	if status.Column != "status" {
		t.Errorf("status column = %q, want status", status.Column)
	}
}

func TestParseColumnTag_PreservesExistingBehavior(t *testing.T) {
	id := r3tag.ParseColumnTag(fieldByName(t, "ID"))
	if id.Column != "id" || !id.IsPK {
		t.Errorf("id = %+v, want column=id pk=true", id)
	}
	del := r3tag.ParseColumnTag(fieldByName(t, "Del"))
	if del.Column != "deleted_at" || !del.SoftDelete {
		t.Errorf("del = %+v, want column=deleted_at soft_delete=true", del)
	}
}
