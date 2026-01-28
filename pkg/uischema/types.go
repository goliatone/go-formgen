package uischema

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Store keeps the parsed operations from UI schema documents. It is safe for
// concurrent readers when treated as immutable after construction.
type Store struct {
	operations map[string]Operation
}

// Operation describes the UI schema overrides for a specific OpenAPI operation.
type Operation struct {
	ID                string
	Source            string
	Form              FormConfig
	Sections          []SectionConfig
	Fields            map[string]FieldConfig
	FieldOrderPresets map[string][]string
}

// FormConfig captures high-level layout instructions plus action buttons.
type FormConfig struct {
	Title       string            `json:"title" yaml:"title"`
	TitleKey    string            `json:"titleKey,omitempty" yaml:"titleKey,omitempty"`
	Subtitle    string            `json:"subtitle" yaml:"subtitle"`
	SubtitleKey string            `json:"subtitleKey,omitempty" yaml:"subtitleKey,omitempty"`
	Layout      LayoutConfig      `json:"layout" yaml:"layout"`
	Actions     []ActionConfig    `json:"actions" yaml:"actions"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
	UIHints     map[string]string `json:"uiHints" yaml:"uiHints"`
}

// LayoutConfig defines the grid used by the renderer.
type LayoutConfig struct {
	GridColumns int    `json:"gridColumns" yaml:"gridColumns"`
	Gutter      string `json:"gutter" yaml:"gutter"`
}

// ActionConfig serialises call-to-action buttons rendered alongside the form.
type ActionConfig struct {
	Kind     string `json:"kind" yaml:"kind"`
	Label    string `json:"label" yaml:"label"`
	LabelKey string `json:"labelKey,omitempty" yaml:"labelKey,omitempty"`
	Href     string `json:"href,omitempty" yaml:"href,omitempty"`
	Type     string `json:"type,omitempty" yaml:"type,omitempty"`
	Icon     string `json:"icon,omitempty" yaml:"icon,omitempty"`
}

// SectionConfig groups related fields into cards/fieldsets.
type SectionConfig struct {
	ID             string            `json:"id" yaml:"id"`
	Title          string            `json:"title" yaml:"title"`
	TitleKey       string            `json:"titleKey,omitempty" yaml:"titleKey,omitempty"`
	Description    string            `json:"description" yaml:"description"`
	DescriptionKey string            `json:"descriptionKey,omitempty" yaml:"descriptionKey,omitempty"`
	Order          *int              `json:"order,omitempty" yaml:"order,omitempty"`
	Fieldset       *bool             `json:"fieldset,omitempty" yaml:"fieldset,omitempty"`
	OrderPreset    OrderPreset       `json:"orderPreset,omitempty" yaml:"orderPreset,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	UIHints        map[string]string `json:"uiHints,omitempty" yaml:"uiHints,omitempty"`
}

// FieldConfig customises how a field is rendered within a section/grid.
type FieldConfig struct {
	Section          string            `json:"section" yaml:"section"`
	Order            *int              `json:"order,omitempty" yaml:"order,omitempty"`
	Grid             *GridConfig       `json:"grid,omitempty" yaml:"grid,omitempty"`
	Label            string            `json:"label,omitempty" yaml:"label,omitempty"`
	LabelKey         string            `json:"labelKey,omitempty" yaml:"labelKey,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
	DescriptionKey   string            `json:"descriptionKey,omitempty" yaml:"descriptionKey,omitempty"`
	HelpText         string            `json:"helpText,omitempty" yaml:"helpText,omitempty"`
	HelpTextKey      string            `json:"helpTextKey,omitempty" yaml:"helpTextKey,omitempty"`
	Placeholder      string            `json:"placeholder,omitempty" yaml:"placeholder,omitempty"`
	PlaceholderKey   string            `json:"placeholderKey,omitempty" yaml:"placeholderKey,omitempty"`
	Widget           string            `json:"widget,omitempty" yaml:"widget,omitempty"`
	Component        string            `json:"component,omitempty" yaml:"component,omitempty"`
	ComponentOptions map[string]any    `json:"componentOptions,omitempty" yaml:"componentOptions,omitempty"`
	Icon             string            `json:"icon,omitempty" yaml:"icon,omitempty"`
	IconSource       string            `json:"iconSource,omitempty" yaml:"iconSource,omitempty"`
	IconRaw          string            `json:"iconRaw,omitempty" yaml:"iconRaw,omitempty"`
	Behaviors        map[string]any    `json:"behaviors,omitempty" yaml:"behaviors,omitempty"`
	CSSClass         string            `json:"cssClass,omitempty" yaml:"cssClass,omitempty"`
	UIHints          map[string]string `json:"uiHints,omitempty" yaml:"uiHints,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	OriginalPath     string            `json:"-" yaml:"-"`
}

// GridConfig describes a field's presence in the layout grid.
type GridConfig struct {
	Span        int                             `json:"span,omitempty" yaml:"span,omitempty"`
	Start       int                             `json:"start,omitempty" yaml:"start,omitempty"`
	Row         int                             `json:"row,omitempty" yaml:"row,omitempty"`
	Breakpoints map[string]GridBreakpointConfig `json:"breakpoints,omitempty" yaml:"breakpoints,omitempty"`
}

// GridBreakpointConfig describes per-breakpoint layout overrides for a field.
type GridBreakpointConfig struct {
	Span  int `json:"span,omitempty" yaml:"span,omitempty"`
	Start int `json:"start,omitempty" yaml:"start,omitempty"`
	Row   int `json:"row,omitempty" yaml:"row,omitempty"`
}

// NormalizeFieldPath converts UI schema field keys into dot/".items" notation.
func NormalizeFieldPath(path string) string {
	if path == "" {
		return ""
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	segments := strings.Split(trimmed, ".")
	normalised := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		parts := normalizeSegment(segment)
		if len(parts) == 0 {
			continue
		}
		normalised = append(normalised, parts...)
	}
	return strings.Join(normalised, ".")
}

func normalizeSegment(segment string) []string {
	if segment == "" {
		return nil
	}
	if !strings.Contains(segment, "[") {
		return []string{segment}
	}

	base := segment
	if idx := strings.Index(segment, "["); idx >= 0 {
		base = segment[:idx]
	}
	tail := ""
	if last := strings.LastIndex(segment, "]"); last >= 0 && last+1 < len(segment) {
		tail = strings.TrimSpace(segment[last+1:])
	}

	count := strings.Count(segment, "[")
	parts := make([]string, 0, count+2)
	if base != "" {
		parts = append(parts, base)
	}
	itemsToAdd := count
	if base == "items" && itemsToAdd > 0 {
		itemsToAdd--
	}
	for i := 0; i < itemsToAdd; i++ {
		parts = append(parts, "items")
	}
	if tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

// OrderPreset represents a per-section ordering preset, which can either refer
// to a named preset declared at the document level or inline an array of field
// paths. Callers should treat zero values as “not configured”.
type OrderPreset struct {
	reference string
	inline    []string
	defined   bool
}

// Defined reports whether the preset has been set in the schema.
func (p OrderPreset) Defined() bool {
	return p.defined
}

// Reference returns the referenced preset name when the schema points at a
// document-level preset. Empty strings indicate either the preset was unset or
// the section uses an inline list instead.
func (p OrderPreset) Reference() string {
	return p.reference
}

// Inline returns a copy of the inline pattern when the schema provided an
// explicit array. The returned slice may be nil when the preset references a
// named preset instead.
func (p OrderPreset) Inline() []string {
	if len(p.inline) == 0 {
		return nil
	}
	out := make([]string, len(p.inline))
	copy(out, p.inline)
	return out
}

// HasInline reports whether the preset contains an inline array.
func (p OrderPreset) HasInline() bool {
	return len(p.inline) > 0
}

// Pattern resolves the preset into a slice of strings, cloning either the
// inline array or the referenced document-level preset. The returned slice is
// safe to mutate.
func (p OrderPreset) Pattern(globals map[string][]string) ([]string, error) {
	if !p.defined {
		return nil, nil
	}
	if len(p.inline) > 0 {
		return p.Inline(), nil
	}
	if p.reference == "" {
		return nil, fmt.Errorf("orderPreset: missing preset name")
	}
	pattern, ok := globals[p.reference]
	if !ok || len(pattern) == 0 {
		return nil, fmt.Errorf("orderPreset: preset %q not defined", p.reference)
	}
	out := make([]string, len(pattern))
	copy(out, pattern)
	return out, nil
}

// UnmarshalJSON accepts either a string (named preset) or an array of strings
// (inline pattern). Any other type results in an error.
func (p *OrderPreset) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*p = OrderPreset{}
		return nil
	}
	switch data[0] {
	case '"':
		var ref string
		if err := json.Unmarshal(data, &ref); err != nil {
			return err
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return fmt.Errorf("orderPreset: reference name cannot be empty")
		}
		p.reference = ref
		p.inline = nil
		p.defined = true
		return nil
	case '[':
		var inline []string
		if err := json.Unmarshal(data, &inline); err != nil {
			return fmt.Errorf("orderPreset: inline pattern must be an array of strings: %w", err)
		}
		return p.assignInline(inline)
	default:
		return fmt.Errorf("orderPreset: expected string or array, got %s", string(data))
	}
}

// UnmarshalYAML mirrors the JSON behaviour to keep YAML parity.
func (p *OrderPreset) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*p = OrderPreset{}
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		var ref string
		if err := value.Decode(&ref); err != nil {
			return err
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return fmt.Errorf("orderPreset: reference name cannot be empty")
		}
		p.reference = ref
		p.inline = nil
		p.defined = true
		return nil
	case yaml.SequenceNode:
		var inline []string
		if err := value.Decode(&inline); err != nil {
			return fmt.Errorf("orderPreset: inline pattern must be an array of strings: %w", err)
		}
		return p.assignInline(inline)
	default:
		return fmt.Errorf("orderPreset: expected string or array, got kind %d", value.Kind)
	}
}

func (p *OrderPreset) assignInline(values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("orderPreset: inline pattern cannot be empty")
	}
	out := make([]string, len(values))
	for idx, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return fmt.Errorf("orderPreset: inline pattern contains an empty entry at index %d", idx)
		}
		out[idx] = trimmed
	}
	p.reference = ""
	p.inline = out
	p.defined = true
	return nil
}
