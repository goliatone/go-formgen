package jsonschema

import "testing"

func TestOverlayApply_MergesExtensions(t *testing.T) {
	raw := []byte(`{
  "type": "object",
  "properties": {
    "title": {
      "type": "string",
      "x-formgen": { "label": "Inline", "widget": "text" }
    }
  }
}`)
	payload, err := parseJSONSchema(raw)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	overlayRaw := []byte(`{
  "$schema": "x-ui-overlay/v1",
  "overrides": [
    {
      "path": "/properties/title",
      "x-formgen": { "label": "Overlay" }
    }
  ]
}`)
	overlay, err := ParseOverlay(overlayRaw)
	if err != nil {
		t.Fatalf("parse overlay: %v", err)
	}
	if err := ApplyOverlay(payload, overlay); err != nil {
		t.Fatalf("apply overlay: %v", err)
	}

	props := payload["properties"].(map[string]any)
	title := props["title"].(map[string]any)
	ext := title["x-formgen"].(map[string]any)
	if got := ext["label"]; got != "Overlay" {
		t.Fatalf("expected overlay label, got %#v", got)
	}
	if got := ext["widget"]; got != "text" {
		t.Fatalf("expected widget preserved, got %#v", got)
	}
}

func TestOverlayApply_InvalidPath(t *testing.T) {
	payload := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
		},
	}
	overlay := Overlay{
		Overrides: []OverlayOverride{
			{
				Path: "/properties/missing",
				Extensions: map[string]any{
					"x-formgen": map[string]any{"label": "Missing"},
				},
			},
		},
	}
	if err := ApplyOverlay(payload, overlay); err == nil {
		t.Fatalf("expected invalid path error")
	}
}
