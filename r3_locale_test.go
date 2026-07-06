package r3_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
)

func TestLocaleContext(t *testing.T) {
	ctx := context.Background()

	if got := r3.GetLocale(ctx); got != "" {
		t.Errorf("GetLocale on bare context = %q, want empty", got)
	}

	ctx = r3.WithLocale(ctx, "ru")
	if got := r3.GetLocale(ctx); got != "ru" {
		t.Errorf("GetLocale = %q, want %q", got, "ru")
	}

	// Overriding replaces the previous value.
	ctx = r3.WithLocale(ctx, "ro")
	if got := r3.GetLocale(ctx); got != "ro" {
		t.Errorf("GetLocale after override = %q, want %q", got, "ro")
	}
}
