package r3_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/expectto/be"
	"github.com/expectto/be/be_string"
)

func TestLocaleContext(t *testing.T) {
	ctx := context.Background()

	be.AssertThat(t, r3.GetLocale(ctx), be_string.EmptyString())

	ctx = r3.WithLocale(ctx, "ru")
	be.AssertThat(t, r3.GetLocale(ctx), be.Eq("ru"))

	// Overriding replaces the previous value.
	ctx = r3.WithLocale(ctx, "ro")
	be.AssertThat(t, r3.GetLocale(ctx), be.Eq("ro"))
}
