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
			"field":                    field,
			"config":                   data.Config,
			"theme":                    data.Theme,
			"value":                    value,
			"schema_hint":              cfg.SchemaHint,
			"collapsed":                cfg.Collapsed,
			"valid":                    valid,
			"example":                  cfg.Example,
			"mode":                     string(cfg.Mode),
			"active_view_from_payload": cfg.ActiveView,
			"show_raw":                 cfg.Mode == JSONEditorModeRaw || cfg.Mode == JSONEditorModeHybrid,
			"show_gui":                 cfg.Mode == JSONEditorModeGUI || cfg.Mode == JSONEditorModeHybrid,
			"show_toggle":              cfg.Mode == JSONEditorModeHybrid,
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
	ActiveView string
}

func parseJSONEditorConfig(field model.Field, cfg map[string]any) jsonEditorConfig {
	config := jsonEditorConfig{
		SchemaHint: strings.TrimSpace(field.Description),
		Example:    strings.TrimSpace(field.Placeholder),
		Mode:       JSONEditorModeRaw,
		ActiveView: "raw",
	}
	applyJSONEditorStringMap(&config, field.UIHints, jsonEditorHintKeys{
		hint: "schemaHint", example: "jsonExample", collapsed: "collapsed", mode: "editorMode", activeView: "editorActiveView",
	})
	applyJSONEditorStringMap(&config, field.Metadata, jsonEditorHintKeys{
		hint: "schema.hint", example: "schema.example", collapsed: "json.collapsed", mode: "editor.mode", activeView: "editor.activeView",
	})
	applyJSONEditorAnyMap(&config, cfg)

	if config.SchemaHint == "" {
		config.SchemaHint = "Provide a JSON object; unknown keys are preserved."
	}
	return config
}

type jsonEditorHintKeys struct {
	hint       string
	example    string
	collapsed  string
	mode       string
	activeView string
}

func applyJSONEditorStringMap(config *jsonEditorConfig, values map[string]string, keys jsonEditorHintKeys) {
	if value := strings.TrimSpace(values[keys.hint]); value != "" {
		config.SchemaHint = value
	}
	if value := strings.TrimSpace(values[keys.example]); value != "" {
		config.Example = value
	}
	if asBool(values[keys.collapsed]) {
		config.Collapsed = true
	}
	if value := strings.TrimSpace(values[keys.mode]); value != "" {
		config.Mode = parseJSONEditorMode(value)
	}
	applyJSONEditorActiveView(config, values[keys.activeView])
}

func applyJSONEditorAnyMap(config *jsonEditorConfig, values map[string]any) {
	if values == nil {
		return
	}
	if value := strings.TrimSpace(stringifyConfigValue(values, "schemaHint")); value != "" {
		config.SchemaHint = value
	}
	if value := strings.TrimSpace(stringifyConfigValue(values, "example")); value != "" {
		config.Example = value
	}
	if asBool(values["collapsed"]) {
		config.Collapsed = true
	}
	if value := strings.TrimSpace(stringifyConfigValue(values, "mode")); value != "" {
		config.Mode = parseJSONEditorMode(value)
	}
	applyJSONEditorActiveView(config, stringifyConfigValue(values, "activeView"))
}

func applyJSONEditorActiveView(config *jsonEditorConfig, value string) {
	if parsed, ok := parseJSONEditorActiveView(strings.TrimSpace(value)); ok {
		config.ActiveView = parsed
	}
}

func stringifyConfigValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	return stringify(value)
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

func parseJSONEditorActiveView(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gui":
		return "gui", true
	case "raw":
		return "raw", true
	default:
		return "", false
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
		return encodeStableMap(buf, typed, depth)
	case []any:
		return encodeStableList(buf, typed, depth)
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

func encodeStableMap(buf *bytes.Buffer, value map[string]any, depth int) error {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buf.WriteByte('{')
	if len(keys) == 0 {
		buf.WriteByte('}')
		return nil
	}
	buf.WriteByte('\n')
	indent := strings.Repeat("  ", depth+1)
	for idx, key := range keys {
		buf.WriteString(indent)
		keyBytes, _ := json.Marshal(key)
		buf.Write(keyBytes)
		buf.WriteString(": ")
		if err := encodeStableJSON(buf, value[key], depth+1); err != nil {
			return err
		}
		writeJSONCommaAndNewline(buf, idx, len(keys))
	}
	buf.WriteString(strings.Repeat("  ", depth))
	buf.WriteByte('}')
	return nil
}

func encodeStableList(buf *bytes.Buffer, value []any, depth int) error {
	buf.WriteByte('[')
	if len(value) == 0 {
		buf.WriteByte(']')
		return nil
	}
	buf.WriteByte('\n')
	indent := strings.Repeat("  ", depth+1)
	for idx, item := range value {
		buf.WriteString(indent)
		if err := encodeStableJSON(buf, item, depth+1); err != nil {
			return err
		}
		writeJSONCommaAndNewline(buf, idx, len(value))
	}
	buf.WriteString(strings.Repeat("  ", depth))
	buf.WriteByte(']')
	return nil
}

func writeJSONCommaAndNewline(buf *bytes.Buffer, idx, count int) {
	if idx < count-1 {
		buf.WriteByte(',')
	}
	buf.WriteByte('\n')
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
