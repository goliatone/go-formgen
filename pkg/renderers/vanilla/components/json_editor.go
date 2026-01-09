package components

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

const (
	jsonEditorTemplate = templatePrefix + "json_editor.tmpl"
	jsonEditorPartial  = "forms.json-editor"
)

func jsonEditorDescriptor() Descriptor {
	return Descriptor{
		Renderer: jsonEditorRenderer(),
		Scripts: []Script{
			{
				Inline: jsonEditorInlineScript,
				Defer:  true,
			},
		},
	}
}

func jsonEditorRenderer() Renderer {
	return func(buf *bytes.Buffer, field model.Field, data ComponentData) error {
		if data.Template == nil {
			return fmt.Errorf("components: template renderer not configured for %q", jsonEditorTemplate)
		}

		templateName := jsonEditorTemplate
		if data.ThemePartials != nil {
			if candidate := strings.TrimSpace(data.ThemePartials[jsonEditorPartial]); candidate != "" {
				templateName = candidate
			}
		}

		cfg := parseJSONEditorConfig(field, data.Config)
		value, valid := jsonEditorValue(field.Default)
		if strings.TrimSpace(value) == "" {
			value = "{}"
			valid = true
		}

		payload := map[string]any{
			"field":       field,
			"config":      data.Config,
			"theme":       data.Theme,
			"value":       value,
			"schema_hint": cfg.SchemaHint,
			"collapsed":   cfg.Collapsed,
			"valid":       valid,
			"example":     cfg.Example,
			"mode":        string(cfg.Mode),
			"show_raw":    cfg.Mode == JSONEditorModeRaw || cfg.Mode == JSONEditorModeHybrid,
			"show_gui":    cfg.Mode == JSONEditorModeGUI || cfg.Mode == JSONEditorModeHybrid,
			"show_toggle": cfg.Mode == JSONEditorModeHybrid,
		}

		rendered, err := data.Template.RenderTemplate(templateName, payload)
		if err != nil {
			return fmt.Errorf("components: render template %q: %w", templateName, err)
		}

		buf.WriteString(rendered)
		return nil
	}
}

// JSONEditorMode defines the editing interface mode.
type JSONEditorMode string

const (
	// JSONEditorModeRaw shows only the raw textarea editor.
	JSONEditorModeRaw JSONEditorMode = "raw"
	// JSONEditorModeGUI shows only the GUI key-value editor.
	JSONEditorModeGUI JSONEditorMode = "gui"
	// JSONEditorModeHybrid shows both with a toggle to switch.
	JSONEditorModeHybrid JSONEditorMode = "hybrid"
)

type jsonEditorConfig struct {
	SchemaHint string
	Collapsed  bool
	Example    string
	Mode       JSONEditorMode
}

func parseJSONEditorConfig(field model.Field, cfg map[string]any) jsonEditorConfig {
	hint := strings.TrimSpace(field.Description)
	example := strings.TrimSpace(field.Placeholder)
	collapsed := false
	mode := JSONEditorModeRaw // Default to raw for backwards compatibility

	if field.UIHints != nil {
		if value := strings.TrimSpace(field.UIHints["schemaHint"]); value != "" {
			hint = value
		}
		if value := strings.TrimSpace(field.UIHints["jsonExample"]); value != "" {
			example = value
		}
		if asBool(field.UIHints["collapsed"]) {
			collapsed = true
		}
		if value := strings.TrimSpace(field.UIHints["editorMode"]); value != "" {
			mode = parseJSONEditorMode(value)
		}
	}
	if field.Metadata != nil {
		if value := strings.TrimSpace(field.Metadata["schema.hint"]); value != "" {
			hint = value
		}
		if value := strings.TrimSpace(field.Metadata["schema.example"]); value != "" {
			example = value
		}
		if asBool(field.Metadata["json.collapsed"]) {
			collapsed = true
		}
		if value := strings.TrimSpace(field.Metadata["editor.mode"]); value != "" {
			mode = parseJSONEditorMode(value)
		}
	}
	if cfg != nil {
		if value := strings.TrimSpace(stringify(cfg["schemaHint"])); value != "" {
			hint = value
		}
		if value := strings.TrimSpace(stringify(cfg["example"])); value != "" {
			example = value
		}
		if asBool(cfg["collapsed"]) {
			collapsed = true
		}
		if value := strings.TrimSpace(stringify(cfg["mode"])); value != "" {
			mode = parseJSONEditorMode(value)
		}
	}

	if hint == "" {
		hint = "Provide a JSON object; unknown keys are preserved."
	}

	return jsonEditorConfig{
		SchemaHint: hint,
		Collapsed:  collapsed,
		Example:    example,
		Mode:       mode,
	}
}

func parseJSONEditorMode(value string) JSONEditorMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gui":
		return JSONEditorModeGUI
	case "hybrid", "both":
		return JSONEditorModeHybrid
	default:
		return JSONEditorModeRaw
	}
}

func jsonEditorValue(value any) (string, bool) {
	if value == nil {
		return "{}", true
	}

	if str, ok := value.(string); ok {
		trimmed := strings.TrimSpace(str)
		if trimmed == "" {
			return "{}", true
		}
		if pretty, ok := stableJSONString(trimmed); ok {
			return pretty, true
		}
		return trimmed, false
	}

	if pretty, ok := stableJSONString(value); ok {
		return pretty, true
	}

	return fmt.Sprint(value), false
}

func stableJSONString(value any) (string, bool) {
	normalized, err := normalizeJSONValue(value)
	if err != nil {
		return "", false
	}

	var buf bytes.Buffer
	if err := encodeStableJSON(&buf, normalized, 0); err != nil {
		return "", false
	}

	return buf.String(), true
}

func normalizeJSONValue(value any) (any, error) {
	switch v := value.(type) {
	case nil:
		return map[string]any{}, nil
	case json.RawMessage:
		if len(v) == 0 {
			return map[string]any{}, nil
		}
		var target any
		if err := json.Unmarshal(v, &target); err != nil {
			return nil, err
		}
		return target, nil
	case []byte:
		return normalizeJSONValue(json.RawMessage(v))
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return map[string]any{}, nil
		}
		var target any
		if err := json.Unmarshal([]byte(trimmed), &target); err != nil {
			return nil, err
		}
		return target, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var target any
		if err := json.Unmarshal(data, &target); err != nil {
			return nil, err
		}
		return target, nil
	}
}

func encodeStableJSON(buf *bytes.Buffer, value any, depth int) error {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		buf.WriteByte('{')
		if len(keys) > 0 {
			buf.WriteByte('\n')
			indent := strings.Repeat("  ", depth+1)
			for idx, key := range keys {
				buf.WriteString(indent)
				keyBytes, _ := json.Marshal(key)
				buf.Write(keyBytes)
				buf.WriteString(": ")
				if err := encodeStableJSON(buf, typed[key], depth+1); err != nil {
					return err
				}
				if idx < len(keys)-1 {
					buf.WriteByte(',')
				}
				buf.WriteByte('\n')
			}
			buf.WriteString(strings.Repeat("  ", depth))
		}
		buf.WriteByte('}')
		return nil
	case []any:
		buf.WriteByte('[')
		if len(typed) > 0 {
			buf.WriteByte('\n')
			indent := strings.Repeat("  ", depth+1)
			for idx, item := range typed {
				buf.WriteString(indent)
				if err := encodeStableJSON(buf, item, depth+1); err != nil {
					return err
				}
				if idx < len(typed)-1 {
					buf.WriteByte(',')
				}
				buf.WriteByte('\n')
			}
			buf.WriteString(strings.Repeat("  ", depth))
		}
		buf.WriteByte(']')
		return nil
	case nil:
		buf.WriteString("null")
		return nil
	default:
		valueBytes, err := json.Marshal(typed)
		if err != nil {
			return err
		}
		buf.Write(valueBytes)
		return nil
	}
}

func stringify(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(v))
		return trimmed == "true" || trimmed == "1" || trimmed == "yes" || trimmed == "on"
	default:
		return false
	}
}

// jsonEditorInlineScript provides a minimal fallback for raw-mode JSON editors
// when the behaviors bundle (formgen-behaviors.min.js) is not loaded.
// The full GUI mode requires the behaviors bundle which includes the TypeScript
// implementation in client/src/editors/json-gui.ts.
const jsonEditorInlineScript = `(function () {
  var ROOT = '[data-json-editor="true"]';
  var INIT_ATTR = "data-json-editor-init";

  function parsed(json) {
    try {
      return JSON.parse(json);
    } catch (e) {
      return null;
    }
  }

  function pretty(value) {
    var parsedValue = parsed(value);
    if (parsedValue === null) {
      return value || "";
    }
    return JSON.stringify(parsedValue, null, 2);
  }

  function initEditors() {
    document.querySelectorAll(ROOT).forEach(function (root) {
      if (root.getAttribute(INIT_ATTR) === "true") {
        return;
      }
      // Skip if full behaviors bundle is handling this
      var mode = root.getAttribute("data-json-editor-mode");
      if (mode === "gui" || mode === "hybrid") {
        // GUI/hybrid modes require the behaviors bundle
        return;
      }
      root.setAttribute(INIT_ATTR, "true");

      var textarea = root.querySelector("[data-json-editor-input]");
      var preview = root.querySelector("[data-json-editor-preview]");
      var toggle = root.querySelector("[data-json-editor-toggle]");
      var format = root.querySelector("[data-json-editor-format]");

      function setState(valid) {
        root.setAttribute("data-json-editor-state", valid ? "valid" : "invalid");
        if (preview) {
          preview.dataset.state = valid ? "valid" : "invalid";
        }
      }

      function syncPreview() {
        if (!textarea) {
          return;
        }
        var raw = textarea.value || "";
        var formatted = pretty(raw || "{}");
        var valid = parsed(raw || "{}") !== null;
        if (preview) {
          preview.textContent = formatted || "{}";
        }
        setState(valid);
      }

      function setCollapsed(state) {
        if (textarea) {
          textarea.classList.toggle("hidden", state);
        }
        if (preview) {
          preview.classList.toggle("hidden", !state);
        }
        root.classList.toggle("json-editor--collapsed", state);
        if (toggle) {
          toggle.setAttribute("aria-expanded", (!state).toString());
          toggle.textContent = state ? "Expand" : "Collapse";
        }
      }

      var collapsed = root.getAttribute("data-json-editor-collapsed") === "true";
      setCollapsed(collapsed);
      syncPreview();

      if (toggle) {
        toggle.addEventListener("click", function (event) {
          event.preventDefault();
          setCollapsed(!root.classList.contains("json-editor--collapsed"));
        });
      }

      if (format && textarea) {
        format.addEventListener("click", function (event) {
          event.preventDefault();
          textarea.value = pretty(textarea.value || "{}");
          setCollapsed(false);
          syncPreview();
        });
      }

      if (textarea) {
        textarea.addEventListener("input", syncPreview);
        textarea.addEventListener("blur", syncPreview);
      }
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initEditors);
  } else {
    initEditors();
  }
})();`
