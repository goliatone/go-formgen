package uischema

import "strings"

// Store keeps the parsed operations from UI schema documents. It is safe for
// concurrent readers when treated as immutable after construction.
type Store struct {
	operations map[string]Operation
}

// Operation describes the UI schema overrides for a specific OpenAPI operation.
type Operation struct {
	ID       string
	Source   string
	Form     FormConfig
	Sections []SectionConfig
	Fields   map[string]FieldConfig
}

// FormConfig captures high-level layout instructions plus action buttons.
type FormConfig struct {
	Title    string            `json:"title" yaml:"title"`
	Subtitle string            `json:"subtitle" yaml:"subtitle"`
	Layout   LayoutConfig      `json:"layout" yaml:"layout"`
	Actions  []ActionConfig    `json:"actions" yaml:"actions"`
	Metadata map[string]string `json:"metadata" yaml:"metadata"`
	UIHints  map[string]string `json:"uiHints" yaml:"uiHints"`
}

// LayoutConfig defines the grid used by the renderer.
type LayoutConfig struct {
	GridColumns int    `json:"gridColumns" yaml:"gridColumns"`
	Gutter      string `json:"gutter" yaml:"gutter"`
}

// ActionConfig serialises call-to-action buttons rendered alongside the form.
type ActionConfig struct {
	Kind  string `json:"kind" yaml:"kind"`
	Label string `json:"label" yaml:"label"`
	Href  string `json:"href,omitempty" yaml:"href,omitempty"`
	Type  string `json:"type,omitempty" yaml:"type,omitempty"`
	Icon  string `json:"icon,omitempty" yaml:"icon,omitempty"`
}

// SectionConfig groups related fields into cards/fieldsets.
type SectionConfig struct {
	ID          string            `json:"id" yaml:"id"`
	Title       string            `json:"title" yaml:"title"`
	Description string            `json:"description" yaml:"description"`
	Order       *int              `json:"order,omitempty" yaml:"order,omitempty"`
	Fieldset    *bool             `json:"fieldset,omitempty" yaml:"fieldset,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	UIHints     map[string]string `json:"uiHints,omitempty" yaml:"uiHints,omitempty"`
}

// FieldConfig customises how a field is rendered within a section/grid.
type FieldConfig struct {
	Section          string            `json:"section" yaml:"section"`
	Order            *int              `json:"order,omitempty" yaml:"order,omitempty"`
	Grid             *GridConfig       `json:"grid,omitempty" yaml:"grid,omitempty"`
	Label            string            `json:"label,omitempty" yaml:"label,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
	HelpText         string            `json:"helpText,omitempty" yaml:"helpText,omitempty"`
	Placeholder      string            `json:"placeholder,omitempty" yaml:"placeholder,omitempty"`
	Widget           string            `json:"widget,omitempty" yaml:"widget,omitempty"`
	Component        string            `json:"component,omitempty" yaml:"component,omitempty"`
	ComponentOptions map[string]any    `json:"componentOptions,omitempty" yaml:"componentOptions,omitempty"`
	CSSClass         string            `json:"cssClass,omitempty" yaml:"cssClass,omitempty"`
	UIHints          map[string]string `json:"uiHints,omitempty" yaml:"uiHints,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	OriginalPath     string            `json:"-" yaml:"-"`
}

// GridConfig describes a field's presence in the layout grid.
type GridConfig struct {
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
	replacer := strings.NewReplacer(
		"[].", ".items.",
		"[]", ".items",
		"[", ".",
		"]", "",
	)
	normalised := replacer.Replace(trimmed)
	for strings.Contains(normalised, ".items.items.items") {
		normalised = strings.ReplaceAll(normalised, ".items.items.items", ".items.items")
	}
	normalised = strings.TrimPrefix(normalised, ".")
	for strings.Contains(normalised, "..") {
		normalised = strings.ReplaceAll(normalised, "..", ".")
	}
	return strings.Trim(normalised, ".")
}
