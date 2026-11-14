package r3url_test

import (
	"net/url"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
)

func TestJSONField_DjangoRejects(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true

	_, err := r3url.ParseDjangoFilters(url.Values{"profile->breed": {"beagle"}}, cfg)
	be.ErrorIs(t, err, r3.ErrJSONFieldNotSupported)
}

func TestJSONField_SortFormatsCarryIt(t *testing.T) {
	age := r3.JSONField("profile", "age").AsNumber()
	for _, format := range []r3url.SortFormat{
		r3url.SortFormatColonDir, r3url.SortFormatSignPrefix, r3url.SortFormatJSON,
	} {
		raw, err := r3url.FormatSort(r3.Sorts{r3.NewSortDescSpec(age)}, format)
		be.NoError(t, err)
		sorts, err := r3url.ParseSort(raw, format)
		be.NoError(t, err)
		be.RequireThat(t, sorts, be.HaveLength(1))
		be.AssertThat(t, sorts[0].Column.String(), be.Eq(age.String()))
		be.AssertThat(t, sorts[0].Direction, be.Eq(r3.SortDirectionDesc))
	}
}
