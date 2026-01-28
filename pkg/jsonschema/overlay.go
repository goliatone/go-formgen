package jsonschema

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const overlaySchemaID = "x-ui-overlay/v1"

// Overlay represents a parsed UI overlay document for JSON Schema inputs.
type Overlay struct {
	Overrides []OverlayOverride
}

// OverlayOverride targets a schema node using a JSON Pointer and supplies
// extension overrides.
type OverlayOverride struct {
	Path       string
	Extensions map[string]any
}

// OverlayError reports malformed overlay documents or invalid override paths.
type OverlayError struct {
	Path    string
	Message string
}

func (e OverlayError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "invalid overlay"
	}
	if strings.TrimSpace(e.Path) == "" {
		return "jsonschema overlay: " + msg
	}
	return fmt.Sprintf("jsonschema overlay: %s (%s)", msg, e.Path)
}

// ParseOverlay parses a raw overlay document and extracts extension overrides.
func ParseOverlay(raw []byte) (Overlay, error) {
	if len(raw) == 0 || len(strings.TrimSpace(string(raw))) == 0 {
		return Overlay{}, OverlayError{Message: "overlay document is empty"}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Overlay{}, OverlayError{Message: fmt.Sprintf("parse overlay: %v", err)}
	}
	if payload == nil {
		return Overlay{}, OverlayError{Message: "overlay document is nil"}
	}

	schema := strings.TrimSpace(readString(payload, "$schema"))
	schema = strings.TrimSuffix(schema, "#")
	if schema == "" {
		return Overlay{}, OverlayError{Message: "$schema is required"}
	}
	if schema != overlaySchemaID {
		return Overlay{}, OverlayError{Message: fmt.Sprintf("unsupported $schema %q", schema)}
	}

	rawOverrides, ok := payload["overrides"]
	if !ok {
		return Overlay{}, nil
	}
	list, ok := rawOverrides.([]any)
	if !ok {
		return Overlay{}, OverlayError{Message: "overrides must be an array"}
	}

	overrides := make([]OverlayOverride, 0, len(list))
	for idx, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			return Overlay{}, OverlayError{Message: fmt.Sprintf("overrides[%d] must be an object", idx)}
		}
		path := strings.TrimSpace(readString(entry, "path"))
		if path == "" {
			return Overlay{}, OverlayError{Message: fmt.Sprintf("overrides[%d].path is required", idx)}
		}

		extensions := make(map[string]any)
		for key, value := range entry {
			switch {
			case key == "path":
				continue
			case key == "x-formgen" || key == "x-admin":
				if _, ok := value.(map[string]any); !ok {
					return Overlay{}, OverlayError{Path: path, Message: fmt.Sprintf("%s must be an object", key)}
				}
				extensions[key] = value
			case strings.HasPrefix(key, "x-formgen-") || strings.HasPrefix(key, "x-admin-"):
				extensions[key] = value
			}
		}

		if len(extensions) == 0 {
			continue
		}

		overrides = append(overrides, OverlayOverride{
			Path:       path,
			Extensions: extensions,
		})
	}

	return Overlay{Overrides: overrides}, nil
}

// ApplyOverlay mutates the resolved schema payload with overlay overrides.
func ApplyOverlay(payload map[string]any, overlay Overlay) error {
	if payload == nil || len(overlay.Overrides) == 0 {
		return nil
	}

	for _, override := range overlay.Overrides {
		target, err := resolveOverlayTarget(payload, override.Path)
		if err != nil {
			return OverlayError{Path: override.Path, Message: err.Error()}
		}
		for key, value := range override.Extensions {
			switch key {
			case "x-formgen", "x-admin":
				overrideMap, _ := value.(map[string]any)
				mergeExtensionMap(target, key, overrideMap)
			default:
				target[key] = value
			}
		}
	}

	return nil
}

func resolveOverlayTarget(root map[string]any, pointer string) (map[string]any, error) {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" {
		return nil, fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(trimmed, "#") {
		trimmed = strings.TrimPrefix(trimmed, "#")
	}
	if trimmed == "" || trimmed == "/" {
		return root, nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		return nil, fmt.Errorf("path must be a JSON pointer")
	}

	current := any(root)
	parts := strings.Split(trimmed, "/")[1:]
	for _, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, fmt.Errorf("invalid json pointer %q", pointer)
		}
		decoded = strings.ReplaceAll(decoded, "~1", "/")
		decoded = strings.ReplaceAll(decoded, "~0", "~")

		switch typed := current.(type) {
		case map[string]any:
			value, ok := typed[decoded]
			if !ok {
				return nil, fmt.Errorf("path not found")
			}
			current = value
		case []any:
			idx, err := toIndex(decoded, len(typed))
			if err != nil {
				return nil, fmt.Errorf("path not found")
			}
			current = typed[idx]
		default:
			return nil, fmt.Errorf("path not found")
		}
	}

	target, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("path does not resolve to an object")
	}
	return target, nil
}

func toIndex(raw string, length int) (int, error) {
	if raw == "" {
		return 0, fmt.Errorf("empty index")
	}
	var idx int
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid index")
		}
		idx = idx*10 + int(r-'0')
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("index out of range")
	}
	return idx, nil
}

func mergeExtensionMap(target map[string]any, key string, override map[string]any) {
	if target == nil || override == nil {
		return
	}
	existing, _ := target[key].(map[string]any)
	if existing == nil {
		cloned := make(map[string]any, len(override))
		for nestedKey, value := range override {
			cloned[nestedKey] = value
		}
		target[key] = cloned
		return
	}
	for nestedKey, value := range override {
		existing[nestedKey] = value
	}
	target[key] = existing
}
