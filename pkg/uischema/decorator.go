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
	layoutSectionKey          = "layout.section"
	layoutOrderKey            = "layout.order"
	layoutSpanKey             = "layout.span"
	layoutStartKey            = "layout.start"
	layoutRowKey              = "layout.row"
	layoutSectionsKey         = "layout.sections"
	layoutFieldOrderPrefix    = "layout.fieldOrder."
	componentConfigKey        = "component.config"
	actionsMetadataKey        = "actions"
	behaviorNamesMetadataKey  = "behavior.names"
	behaviorConfigMetadataKey = "behavior.config"
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

	explicitOrders := make(map[string]int, len(op.Fields))
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
			explicitOrders[path] = *cfg.Order
			field.Metadata = ensureMetadata(field.Metadata)
			field.Metadata[layoutOrderKey] = strconv.Itoa(*cfg.Order)
		}

		if err := applyGridHints(field, cfg, maxColumns, op); err != nil {
			return err
		}

		applyFieldCopy(field, cfg)
		mergeFieldMaps(field, cfg)

		if err := applyBehaviorMetadata(field, cfg); err != nil {
			return fmt.Errorf("uischema: operation %q (file %s) field %q: %w", op.ID, op.Source, cfg.OriginalPath, err)
		}
	}

	presetOrders, err := buildSectionFieldOrders(form, op, fieldRefs, originalOrder, explicitOrders)
	if err != nil {
		return err
	}

	reorderFields(form.Fields, "", explicitOrders, presetOrders, originalOrder)
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

func reorderFields(fields []pkgmodel.Field, parentPath string, explicitOrders, presetOrders map[string]int, originals map[string]int) {
	sort.SliceStable(fields, func(i, j int) bool {
		pathI := joinPath(parentPath, fields[i].Name)
		pathJ := joinPath(parentPath, fields[j].Name)

		return fieldOrderLess(pathI, pathJ, explicitOrders, presetOrders, originals)
	})

	for idx := range fields {
		path := joinPath(parentPath, fields[idx].Name)
		if len(fields[idx].Nested) > 0 {
			reorderFields(fields[idx].Nested, path, explicitOrders, presetOrders, originals)
		}
		if fields[idx].Items != nil {
			reorderFieldItems(fields[idx].Items, path, explicitOrders, presetOrders, originals)
		}
	}
}

func reorderFieldItems(field *pkgmodel.Field, parentPath string, explicitOrders, presetOrders map[string]int, originals map[string]int) {
	itemPath := joinPath(parentPath, "items")
	if field == nil {
		return
	}
	if len(field.Nested) > 0 {
		reorderFields(field.Nested, itemPath, explicitOrders, presetOrders, originals)
	}
}

func fieldOrderLess(pathI, pathJ string, explicitOrders, presetOrders map[string]int, originals map[string]int) bool {
	orderI, hasExplicitI := explicitOrders[pathI]
	orderJ, hasExplicitJ := explicitOrders[pathJ]

	switch {
	case hasExplicitI && hasExplicitJ:
		if orderI != orderJ {
			return orderI < orderJ
		}
		return originals[pathI] < originals[pathJ]
	case hasExplicitI:
		return true
	case hasExplicitJ:
		return false
	}

	presetI, hasPresetI := presetOrders[pathI]
	presetJ, hasPresetJ := presetOrders[pathJ]

	switch {
	case hasPresetI && hasPresetJ:
		if presetI != presetJ {
			return presetI < presetJ
		}
		return originals[pathI] < originals[pathJ]
	case hasPresetI:
		return true
	case hasPresetJ:
		return false
	}

	return originals[pathI] < originals[pathJ]
}

func buildSectionFieldOrders(form *pkgmodel.FormModel, op Operation, fieldRefs map[string]*pkgmodel.Field, originals map[string]int, explicitOrders map[string]int) (map[string]int, error) {
	if len(op.Sections) == 0 {
		return nil, nil
	}

	sectionFields := collectSectionFields(fieldRefs)
	if len(sectionFields) == 0 {
		return nil, nil
	}

	presetOrders := make(map[string]int)

	for _, section := range op.Sections {
		id := strings.TrimSpace(section.ID)
		if id == "" {
			continue
		}
		paths := sectionFields[id]
		if len(paths) == 0 || !section.OrderPreset.Defined() {
			continue
		}

		pattern, err := section.OrderPreset.Pattern(op.FieldOrderPresets)
		if err != nil {
			return nil, fmt.Errorf("uischema: operation %q (file %s) section %q: %w", op.ID, op.Source, id, err)
		}
		if len(pattern) == 0 {
			continue
		}

		resolved, err := resolveSectionOrder(pattern, paths, originals)
		if err != nil {
			return nil, fmt.Errorf("uischema: operation %q (file %s) section %q: %w", op.ID, op.Source, id, err)
		}
		if len(resolved) == 0 {
			continue
		}

		sectionPresetOrders := make(map[string]int, len(resolved))
		for idx, path := range resolved {
			sectionPresetOrders[path] = idx
			if _, exists := presetOrders[path]; !exists {
				presetOrders[path] = idx
			}
		}

		ordered := append([]string(nil), paths...)
		sort.SliceStable(ordered, func(i, j int) bool {
			return fieldOrderLess(ordered[i], ordered[j], explicitOrders, sectionPresetOrders, originals)
		})
		if len(ordered) == 0 {
			continue
		}

		form.Metadata = ensureMetadata(form.Metadata)
		payload, err := json.Marshal(ordered)
		if err != nil {
			return nil, fmt.Errorf("uischema: marshal field order metadata for section %q: %w", id, err)
		}
		form.Metadata[layoutFieldOrderPrefix+id] = string(payload)
	}

	if len(presetOrders) == 0 {
		return nil, nil
	}
	return presetOrders, nil
}

func collectSectionFields(fieldRefs map[string]*pkgmodel.Field) map[string][]string {
	sections := make(map[string][]string)
	for path, field := range fieldRefs {
		if field == nil || len(field.Metadata) == 0 {
			continue
		}
		sectionID := strings.TrimSpace(field.Metadata[layoutSectionKey])
		if sectionID == "" {
			continue
		}
		sections[sectionID] = append(sections[sectionID], path)
	}
	return sections
}

func resolveSectionOrder(pattern []string, sectionFields []string, originals map[string]int) ([]string, error) {
	if len(pattern) == 0 {
		return nil, nil
	}

	allowed := make(map[string]struct{}, len(sectionFields))
	for _, path := range sectionFields {
		allowed[path] = struct{}{}
	}

	type token struct {
		wildcard bool
		path     string
	}

	tokens := make([]token, 0, len(pattern))
	explicit := make(map[string]struct{})

	for _, raw := range pattern {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			tokens = append(tokens, token{wildcard: true})
			continue
		}
		normalised := NormalizeFieldPath(trimmed)
		if normalised == "" {
			return nil, fmt.Errorf("field order entry %q resolves to empty path", raw)
		}
		if _, ok := allowed[normalised]; !ok {
			return nil, fmt.Errorf("references unknown field %q", trimmed)
		}
		tokens = append(tokens, token{path: normalised})
		explicit[normalised] = struct{}{}
	}

	if len(tokens) == 0 {
		// No usable tokens; fall back to the natural order.
		out := append([]string(nil), sectionFields...)
		sort.SliceStable(out, func(i, j int) bool {
			return originals[out[i]] < originals[out[j]]
		})
		return out, nil
	}

	ordered := append([]string(nil), sectionFields...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return originals[ordered[i]] < originals[ordered[j]]
	})

	used := make(map[string]bool, len(sectionFields))
	resolved := make([]string, 0, len(sectionFields))

	appendResidual := func(skipExplicit bool) {
		for _, path := range ordered {
			if used[path] {
				continue
			}
			if skipExplicit {
				if _, exists := explicit[path]; exists {
					continue
				}
			}
			used[path] = true
			resolved = append(resolved, path)
		}
	}

	for _, token := range tokens {
		if token.wildcard {
			appendResidual(true)
			continue
		}
		if used[token.path] {
			continue
		}
		used[token.path] = true
		resolved = append(resolved, token.path)
	}

	appendResidual(false)
	return resolved, nil
}

type behaviorMetadataPayload struct {
	names  string
	config string
}

func applyBehaviorMetadata(field *pkgmodel.Field, cfg FieldConfig) error {
	payload, err := buildBehaviorMetadata(cfg)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}
	field.Metadata = ensureMetadata(field.Metadata)
	field.Metadata[behaviorNamesMetadataKey] = payload.names
	field.Metadata[behaviorConfigMetadataKey] = payload.config
	return nil
}

func buildBehaviorMetadata(cfg FieldConfig) (*behaviorMetadataPayload, error) {
	definitions, err := collectBehaviorDefinitions(cfg)
	if err != nil {
		return nil, err
	}
	if len(definitions) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(definitions))
	configs := make(map[string]json.RawMessage, len(definitions))

	for key, value := range definitions {
		payload, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal behavior %q config: %w", key, err)
		}
		names = append(names, key)
		configs[key] = payload
	}

	sort.Strings(names)

	var configJSON string
	if len(names) == 1 {
		configJSON = string(configs[names[0]])
	} else {
		ordered := make(map[string]json.RawMessage, len(names))
		for _, name := range names {
			ordered[name] = configs[name]
		}
		payload, err := json.Marshal(ordered)
		if err != nil {
			return nil, fmt.Errorf("marshal behavior configs: %w", err)
		}
		configJSON = string(payload)
	}

	return &behaviorMetadataPayload{
		names:  strings.Join(names, " "),
		config: configJSON,
	}, nil
}

func collectBehaviorDefinitions(cfg FieldConfig) (map[string]any, error) {
	var merged map[string]any

	if len(cfg.ComponentOptions) > 0 {
		if raw, ok := cfg.ComponentOptions["behaviors"]; ok {
			defs, err := normalizeBehaviorDefinition(raw)
			if err != nil {
				return nil, fmt.Errorf("componentOptions.behaviors: %w", err)
			}
			merged = mergeBehaviorMaps(merged, defs)
		}
	}

	if len(cfg.Behaviors) > 0 {
		defs, err := normalizeBehaviorDefinition(cfg.Behaviors)
		if err != nil {
			return nil, fmt.Errorf("behaviors: %w", err)
		}
		merged = mergeBehaviorMaps(merged, defs)
	}

	return merged, nil
}

func normalizeBehaviorDefinition(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}

	switch typed := value.(type) {
	case map[string]any:
		return cloneBehaviorMap(typed)
	case map[string]string:
		copy := make(map[string]any, len(typed))
		for key, val := range typed {
			copy[key] = val
		}
		return cloneBehaviorMap(copy)
	case map[string]json.RawMessage:
		copy := make(map[string]any, len(typed))
		for key, val := range typed {
			copy[key] = val
		}
		return cloneBehaviorMap(copy)
	default:
		return nil, fmt.Errorf("value of type %T must be an object", value)
	}
}

func cloneBehaviorMap(src map[string]any) (map[string]any, error) {
	if len(src) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		name := strings.TrimSpace(key)
		if name == "" {
			return nil, fmt.Errorf("behavior name cannot be empty")
		}
		out[name] = value
	}
	return out, nil
}

func mergeBehaviorMaps(dst, src map[string]any) map[string]any {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for key, value := range src {
		dst[key] = value
	}
	return dst
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
