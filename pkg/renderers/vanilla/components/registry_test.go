package components

import (
	"bytes"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestRegistryDescriptorClone(t *testing.T) {
	reg := New()
	renderer := func(buf *bytes.Buffer, field model.Field, data ComponentData) error { return nil }

	if err := reg.Register("test", Descriptor{Renderer: renderer, Stylesheets: []string{"/a.css"}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	desc, ok := reg.Descriptor("test")
	if !ok {
		t.Fatalf("descriptor not found")
	}

	desc.Stylesheets = append(desc.Stylesheets, "/mutated.css")

	original, _ := reg.Descriptor("test")
	if len(original.Stylesheets) != 1 || original.Stylesheets[0] != "/a.css" {
		t.Fatalf("registry descriptor mutated: %#v", original.Stylesheets)
	}
}

func TestRegistryAssetsDeduplicates(t *testing.T) {
	reg := New()
	renderer := func(buf *bytes.Buffer, field model.Field, data ComponentData) error { return nil }

	reg.MustRegister("input", Descriptor{
		Renderer:    renderer,
		Stylesheets: []string{"/shared.css", "/input.css"},
		Scripts: []Script{
			{Src: "/shared.js"},
		},
	})
	reg.MustRegister("select", Descriptor{
		Renderer:    renderer,
		Stylesheets: []string{"/shared.css", "/select.css"},
		Scripts: []Script{
			{Src: "/shared.js"},
			{Src: "/select.js"},
		},
	})

	styles, scripts := reg.Assets([]string{"input", "select"})
	if len(styles) != 3 {
		t.Fatalf("expected 3 unique stylesheets, got %d: %v", len(styles), styles)
	}
	if styles[0] != "/input.css" && styles[0] != "/shared.css" {
		// Ordering is not strictly defined but should include unique entries.
		t.Fatalf("unexpected styles ordering: %v", styles)
	}
	if len(scripts) != 2 {
		t.Fatalf("expected 2 unique scripts, got %d: %v", len(scripts), scripts)
	}
}
