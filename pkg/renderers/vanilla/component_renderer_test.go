package vanilla

import (
	"testing"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/renderers/vanilla/components"
)

func TestComponentRendererUnknownComponent(t *testing.T) {
	renderer := newComponentRenderer(nil, components.NewDefaultRegistry(), map[string]string{
		"field": "missing",
	})

	_, err := renderer.render(model.Field{Name: "field"}, "field")
	if err == nil {
		t.Fatalf("expected error when component is missing")
	}

	if got := err.Error(); got != `component "missing" not registered for field "field"` {
		t.Fatalf("unexpected error: %s", got)
	}
}
