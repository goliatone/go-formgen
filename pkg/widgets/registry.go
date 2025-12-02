package widgets

import (
	"sort"
	"strings"
	"sync"

	"github.com/goliatone/go-formgen/pkg/model"
)

// Built-in widget identifiers exposed by the registry.
const (
	WidgetToggle     = "toggle"
	WidgetSelect     = "select"
	WidgetChips      = "chips"
	WidgetCodeEditor = "code-editor"
	WidgetJSONEditor = "json-editor"
	WidgetKeyValue   = "key-value"
)

// Matcher decides whether a widget renderer should handle the supplied field.
type Matcher func(field model.Field) bool

type rule struct {
	name     string
	priority int
	match    Matcher
	order    int
}

// Registry selects widget renderers for fields based on explicit hints or
// registered matchers. Higher priority wins; ties fall back to registration
// order. An empty registry never resolves a widget.
type Registry struct {
	mu    sync.RWMutex
	rules []rule
}

// NewRegistry constructs a registry with the built-in widget matchers
// registered.
func NewRegistry() *Registry {
	reg := &Registry{}
	reg.registerBuiltins()
	return reg
}

// Register adds a widget matcher with the provided name and priority. Higher
// priority values take precedence. Callers should avoid duplicate names; the
// latest registration wins during resolution.
func (r *Registry) Register(name string, priority int, matcher Matcher) {
	if r == nil || matcher == nil {
		return
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rules = append(r.rules, rule{
		name:     trimmed,
		priority: priority,
		match:    matcher,
		order:    len(r.rules),
	})
}

// Resolve returns the widget name for a field. Explicit hints (admin/widget
// metadata or UI hints) are honoured before matcher evaluation.
func (r *Registry) Resolve(field model.Field) (string, bool) {
	if explicit := explicitWidget(field); explicit != "" {
		return explicit, true
	}
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	if len(r.rules) == 0 {
		r.mu.RUnlock()
		return "", false
	}
	rules := append([]rule(nil), r.rules...)
	r.mu.RUnlock()
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].priority == rules[j].priority {
			return rules[i].order < rules[j].order
		}
		return rules[i].priority > rules[j].priority
	})
	for _, entry := range rules {
		if entry.match(field) {
			return entry.name, true
		}
	}
	return "", false
}

// Decorate implements model.Decorator, applying registry resolution to every
// field in the form. When a widget is resolved, both Metadata["widget"] and
// UIHints["widget"] are set to the chosen name, preserving existing values
// when present.
func (r *Registry) Decorate(form *model.FormModel) error {
	if r == nil || form == nil {
		return nil
	}
	form.Fields = r.decorateFields(form.Fields)
	return nil
}

func (r *Registry) decorateFields(fields []model.Field) []model.Field {
	if len(fields) == 0 {
		return fields
	}
	decorated := make([]model.Field, len(fields))
	for idx, field := range fields {
		field = r.decorateField(field)
		decorated[idx] = field
	}
	return decorated
}

func (r *Registry) decorateField(field model.Field) model.Field {
	if widget, ok := r.Resolve(field); ok && widget != "" {
		if field.Metadata == nil {
			field.Metadata = make(map[string]string)
		}
		if field.Metadata["widget"] == "" {
			field.Metadata["widget"] = widget
		}
		if field.UIHints == nil {
			field.UIHints = make(map[string]string)
		}
		if field.UIHints["widget"] == "" {
			field.UIHints["widget"] = widget
		}
	}

	if field.Items != nil {
		item := r.decorateField(*field.Items)
		field.Items = &item
	}
	if len(field.Nested) > 0 {
		field.Nested = r.decorateFields(field.Nested)
	}
	return field
}

func explicitWidget(field model.Field) string {
	if field.Metadata != nil {
		if widget := strings.TrimSpace(field.Metadata["admin.widget"]); widget != "" {
			return widget
		}
		if widget := strings.TrimSpace(field.Metadata["widget"]); widget != "" {
			return widget
		}
	}
	if field.UIHints != nil {
		if widget := strings.TrimSpace(field.UIHints["widget"]); widget != "" {
			return widget
		}
	}
	return ""
}

func (r *Registry) registerBuiltins() {
	r.Register(WidgetToggle, 90, func(field model.Field) bool {
		return field.Type == model.FieldTypeBoolean
	})

	r.Register(WidgetChips, 80, func(field model.Field) bool {
		if field.Type != model.FieldTypeArray {
			return false
		}
		if len(field.Enum) > 0 {
			return true
		}
		if field.Metadata != nil {
			if mode := strings.TrimSpace(field.Metadata["relationship.endpoint.mode"]); mode != "" {
				return true
			}
			if _, ok := field.Metadata["relationship.endpoint.url"]; ok {
				return true
			}
		}
		return false
	})

	r.Register(WidgetSelect, 70, func(field model.Field) bool {
		if field.Type == model.FieldTypeArray || field.Type == model.FieldTypeObject {
			return false
		}
		return len(field.Enum) > 0 || field.Relationship != nil
	})

	r.Register(WidgetCodeEditor, 60, func(field model.Field) bool {
		if field.Type != model.FieldTypeString {
			return false
		}
		format := strings.TrimSpace(strings.ToLower(field.Format))
		return format == "json" || format == "yaml" || format == "toml"
	})

	r.Register(WidgetJSONEditor, 50, func(field model.Field) bool {
		if field.Type != model.FieldTypeObject {
			return false
		}
		if field.Relationship != nil {
			return false
		}
		return len(field.Nested) == 0
	})

	r.Register(WidgetKeyValue, 40, func(field model.Field) bool {
		if field.Type != model.FieldTypeArray || field.Items == nil {
			return false
		}
		if field.Items.Type == model.FieldTypeObject && len(field.Items.Nested) == 0 {
			return true
		}
		return false
	})
}
