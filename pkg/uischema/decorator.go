package uischema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
)

const (
	layoutSectionKey   = "layout.section"
	layoutOrderKey     = "layout.order"
	layoutSpanKey      = "layout.span"
	layoutStartKey     = "layout.start"
	layoutRowKey       = "layout.row"
	layoutSectionsKey  = "layout.sections"
	componentConfigKey = "component.config"
	actionsMetadataKey = "actions"
)

// Decorator applies UI schema metadata to a form model.
type Decorator struct {
	store *Store
}

// NewDecorator builds a Decorator backed by the provided store. When store is
// nil or empty, the decorator becomes a no-op.
func NewDecorator(store *Store) *Decorator {
	return &Decorator{store: store}
}

// Decorate augments the supplied form model with UI schema metadata. When no
// matching operation is found the form is left untouched.
func (d *Decorator) Decorate(form *pkgmodel.FormModel) error {
	if d == nil || d.store == nil || d.store.Empty() || form == nil {
		return nil
	}

	op, ok := d.store.Operation(form.OperationID)
	if !ok {
		return nil
	}

	if err := applyFormConfig(form, op); err != nil {
		return err
	}
	return applyFieldConfig(form, op)
}

func applyFormConfig(form *pkgmodel.FormModel, op Operation) error {
	form.Metadata = mergeStringMap(form.Metadata, op.Form.Metadata)
	form.UIHints = mergeStringMap(form.UIHints, op.Form.UIHints)

	if op.Form.Title != "" {
		form.UIHints = ensureUIHints(form.UIHints)
		form.UIHints["layout.title"] = op.Form.Title
	}
	if op.Form.Subtitle != "" {
		form.UIHints = ensureUIHints(form.UIHints)
		form.UIHints["layout.subtitle"] = op.Form.Subtitle
	}
	if op.Form.Layout.GridColumns > 0 {
		form.UIHints = ensureUIHints(form.UIHints)
		form.UIHints["layout.gridColumns"] = strconv.Itoa(op.Form.Layout.GridColumns)
	}
	if op.Form.Layout.Gutter != "" {
		form.UIHints = ensureUIHints(form.UIHints)
		form.UIHints["layout.gutter"] = op.Form.Layout.Gutter
	}
	if len(op.Form.Actions) > 0 {
		form.Metadata = ensureMetadata(form.Metadata)
		payload, err := json.Marshal(op.Form.Actions)
		if err != nil {
			return fmt.Errorf("uischema: marshal actions for operation %q: %w", op.ID, err)
		}
		form.Metadata[actionsMetadataKey] = string(payload)
	}

	if len(op.Sections) > 0 {
		exported, err := buildSectionsMetadata(op)
		if err != nil {
			return err
		}
		form.Metadata = ensureMetadata(form.Metadata)
		form.Metadata[layoutSectionsKey] = exported
	}

	return nil
}

func buildSectionsMetadata(op Operation) (string, error) {
	sections := make([]sectionMetadata, 0, len(op.Sections))
	seen := make(map[string]struct{}, len(op.Sections))

	for idx, section := range op.Sections {
		id := strings.TrimSpace(section.ID)
		if id == "" {
			return "", fmt.Errorf("uischema: operation %q (file %s) defines a section without id", op.ID, op.Source)
		}
		if _, exists := seen[id]; exists {
			return "", fmt.Errorf("uischema: operation %q (file %s) defines duplicate section id %q", op.ID, op.Source, id)
		}
		seen[id] = struct{}{}

		order := idx
		if section.Order != nil {
			order = *section.Order
		}
		fieldset := false
		if section.Fieldset != nil {
			fieldset = *section.Fieldset
		}

		entry := sectionMetadata{
			ID:          id,
			Title:       section.Title,
			Description: section.Description,
			Order:       order,
			Fieldset:    fieldset,
			Metadata:    cloneStringMap(section.Metadata),
			UIHints:     cloneStringMap(section.UIHints),
		}
		sections = append(sections, entry)
	}

	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Order != sections[j].Order {
			return sections[i].Order < sections[j].Order
		}
		return sections[i].ID < sections[j].ID
	})

	payload, err := json.Marshal(sections)
	if err != nil {
		return "", fmt.Errorf("uischema: marshal sections for operation %q: %w", op.ID, err)
	}
	return string(payload), nil
}

type sectionMetadata struct {
	ID          string            `json:"id"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Order       int               `json:"order"`
	Fieldset    bool              `json:"fieldset,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	UIHints     map[string]string `json:"uiHints,omitempty"`
}

func applyFieldConfig(form *pkgmodel.FormModel, op Operation) error {
	fieldRefs := make(map[string]*pkgmodel.Field)
	originalOrder := make(map[string]int)
	collectFieldRefs(form.Fields, "", fieldRefs, originalOrder)

	sectionLookup := make(map[string]SectionConfig, len(op.Sections))
	for _, section := range op.Sections {
		if trimmed := strings.TrimSpace(section.ID); trimmed != "" {
			sectionLookup[trimmed] = section
		}
	}

	targetOrders := make(map[string]int, len(op.Fields))
	maxColumns := op.Form.Layout.GridColumns
	if maxColumns <= 0 {
		maxColumns = 12
	}

	for path, cfg := range op.Fields {
		field, ok := fieldRefs[path]
		if !ok {
			return fmt.Errorf("uischema: operation %q (file %s) references unknown field %q", op.ID, op.Source, cfg.OriginalPath)
		}

		if cfg.Section != "" {
			if _, exists := sectionLookup[cfg.Section]; !exists && len(op.Sections) > 0 {
				return fmt.Errorf("uischema: operation %q (file %s) field %q references unknown section %q", op.ID, op.Source, cfg.OriginalPath, cfg.Section)
			}
			field.Metadata = ensureMetadata(field.Metadata)
			field.Metadata[layoutSectionKey] = cfg.Section
		}

		if cfg.Order != nil {
			targetOrders[path] = *cfg.Order
			field.Metadata = ensureMetadata(field.Metadata)
			field.Metadata[layoutOrderKey] = strconv.Itoa(*cfg.Order)
		}

		if err := applyGridHints(field, cfg, maxColumns, op); err != nil {
			return err
		}

		applyFieldCopy(field, cfg)
		mergeFieldMaps(field, cfg)
	}

	reorderFields(form.Fields, "", targetOrders, originalOrder)
	return nil
}

func applyGridHints(field *pkgmodel.Field, cfg FieldConfig, columns int, op Operation) error {
	if cfg.Grid == nil {
		return nil
	}

	span := cfg.Grid.Span
	if span <= 0 {
		if columns > 0 {
			span = columns
		} else {
			span = 12
		}
	}
	if columns > 0 && span > columns {
		return fmt.Errorf("uischema: operation %q (file %s) field %q grid span %d exceeds layout columns %d", op.ID, op.Source, cfg.OriginalPath, span, columns)
	}

	field.UIHints = ensureUIHints(field.UIHints)
	field.UIHints[layoutSpanKey] = strconv.Itoa(span)

	if cfg.Grid.Start > 0 {
		field.UIHints[layoutStartKey] = strconv.Itoa(cfg.Grid.Start)
	}
	if cfg.Grid.Row > 0 {
		field.UIHints[layoutRowKey] = strconv.Itoa(cfg.Grid.Row)
	}

	return nil
}

func applyFieldCopy(field *pkgmodel.Field, cfg FieldConfig) {
	if cfg.Label != "" {
		field.Label = cfg.Label
	}
	if cfg.Description != "" {
		field.Description = cfg.Description
	}
	if cfg.Placeholder != "" {
		field.Placeholder = cfg.Placeholder
	}
	if cfg.HelpText != "" {
		field.UIHints = ensureUIHints(field.UIHints)
		field.UIHints["helpText"] = cfg.HelpText
	}
	if cfg.Widget != "" {
		field.UIHints = ensureUIHints(field.UIHints)
		field.UIHints["widget"] = cfg.Widget
	}
	if cfg.Component != "" {
		field.UIHints = ensureUIHints(field.UIHints)
		field.UIHints["component"] = cfg.Component
	}
	if cfg.CSSClass != "" {
		field.UIHints = ensureUIHints(field.UIHints)
		field.UIHints["cssClass"] = cfg.CSSClass
	}
	if len(cfg.ComponentOptions) > 0 {
		field.Metadata = ensureMetadata(field.Metadata)
		payload, err := json.Marshal(cfg.ComponentOptions)
		if err == nil {
			field.Metadata[componentConfigKey] = string(payload)
		}
	}
}

func mergeFieldMaps(field *pkgmodel.Field, cfg FieldConfig) {
	if len(cfg.UIHints) > 0 {
		field.UIHints = ensureUIHints(field.UIHints)
		for key, value := range cfg.UIHints {
			field.UIHints[key] = value
		}
	}
	if len(cfg.Metadata) > 0 {
		field.Metadata = ensureMetadata(field.Metadata)
		for key, value := range cfg.Metadata {
			field.Metadata[key] = value
		}
	}
}

func reorderFields(fields []pkgmodel.Field, parentPath string, targetOrders map[string]int, originals map[string]int) {
	sort.SliceStable(fields, func(i, j int) bool {
		pathI := joinPath(parentPath, fields[i].Name)
		pathJ := joinPath(parentPath, fields[j].Name)

		orderI, hasI := targetOrders[pathI]
		orderJ, hasJ := targetOrders[pathJ]

		switch {
		case hasI && hasJ:
			if orderI != orderJ {
				return orderI < orderJ
			}
			return originals[pathI] < originals[pathJ]
		case hasI:
			return true
		case hasJ:
			return false
		default:
			return originals[pathI] < originals[pathJ]
		}
	})

	for idx := range fields {
		path := joinPath(parentPath, fields[idx].Name)
		if len(fields[idx].Nested) > 0 {
			reorderFields(fields[idx].Nested, path, targetOrders, originals)
		}
		if fields[idx].Items != nil {
			reorderFieldItems(fields[idx].Items, path, targetOrders, originals)
		}
	}
}

func reorderFieldItems(field *pkgmodel.Field, parentPath string, targetOrders map[string]int, originals map[string]int) {
	itemPath := joinPath(parentPath, "items")
	if field == nil {
		return
	}
	if len(field.Nested) > 0 {
		reorderFields(field.Nested, itemPath, targetOrders, originals)
	}
}

func collectFieldRefs(fields []pkgmodel.Field, parentPath string, refs map[string]*pkgmodel.Field, originals map[string]int) {
	for idx := range fields {
		field := &fields[idx]
		path := joinPath(parentPath, field.Name)
		refs[path] = field
		if _, exists := originals[path]; !exists {
			originals[path] = idx
		}

		if len(field.Nested) > 0 {
			collectFieldRefs(field.Nested, path, refs, originals)
		}

		if field.Items != nil {
			itemPath := joinPath(path, "items")
			refs[itemPath] = field.Items
			if _, exists := originals[itemPath]; !exists {
				originals[itemPath] = 0
			}
			if len(field.Items.Nested) > 0 {
				collectFieldRefs(field.Items.Nested, itemPath, refs, originals)
			}
		}
	}
}

func ensureMetadata(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

func ensureUIHints(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

func mergeStringMap(dst, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}
