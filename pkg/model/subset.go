package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	layoutSectionsKey      = "layout.sections"
	layoutFieldOrderPrefix = "layout.fieldOrder."
	layoutSectionFieldKey  = "layout.section"
)

// FieldSubset describes the allowed groups, tags, or sections for partial
// model output. When all slices are empty the form is left untouched.
type FieldSubset struct {
	Groups   []string
	Tags     []string
	Sections []string
}

// ApplySubset removes fields that do not match the supplied subset filters. It
// operates on top-level fields and prunes section metadata so consumers do not
// render empty sections after filtering. When subset is empty or form is nil,
// the form is returned unchanged.
func ApplySubset(form *FormModel, subset FieldSubset) {
	if form == nil {
		return
	}

	matcher := newSubsetMatcher(subset)
	if matcher.empty() {
		return
	}

	filtered := make([]Field, 0, len(form.Fields))
	for _, field := range form.Fields {
		if matcher.matches(field) {
			filtered = append(filtered, field)
		}
	}
	form.Fields = filtered
	if len(form.Fields) == 0 {
		form.Fields = nil
	}

	pruneSectionMetadata(form, form.Fields)
}

type subsetMatcher struct {
	groups   map[string]struct{}
	tags     map[string]struct{}
	sections map[string]struct{}
}

func newSubsetMatcher(subset FieldSubset) subsetMatcher {
	return subsetMatcher{
		groups:   normaliseTokens(subset.Groups),
		tags:     normaliseTokens(subset.Tags),
		sections: normaliseTokens(subset.Sections),
	}
}

func (m subsetMatcher) empty() bool {
	return len(m.groups) == 0 && len(m.tags) == 0 && len(m.sections) == 0
}

func (m subsetMatcher) matches(field Field) bool {
	if len(m.groups) > 0 {
		if group := normaliseToken(fieldGroup(field)); group != "" {
			if _, ok := m.groups[group]; ok {
				return true
			}
		}
	}

	if len(m.tags) > 0 {
		tags := fieldTags(field)
		for _, tag := range tags {
			if _, ok := m.tags[tag]; ok {
				return true
			}
		}
	}

	if len(m.sections) > 0 {
		if section := normaliseToken(fieldSection(field)); section != "" {
			if _, ok := m.sections[section]; ok {
				return true
			}
		}
	}

	return false
}

func fieldGroup(field Field) string {
	if field.Metadata != nil {
		if candidate := strings.TrimSpace(field.Metadata["group"]); candidate != "" {
			return candidate
		}
		if candidate := strings.TrimSpace(field.Metadata["admin.group"]); candidate != "" {
			return candidate
		}
	}
	if field.UIHints != nil {
		if candidate := strings.TrimSpace(field.UIHints["group"]); candidate != "" {
			return candidate
		}
		if candidate := strings.TrimSpace(field.UIHints["admin.group"]); candidate != "" {
			return candidate
		}
	}
	return ""
}

func fieldTags(field Field) []string {
	collect := func(raw string) []string {
		return parseTokenList(raw)
	}

	var tags []string
	if field.Metadata != nil {
		tags = append(tags, collect(field.Metadata["tags"])...)
		tags = append(tags, collect(field.Metadata["admin.tags"])...)
	}
	if field.UIHints != nil {
		tags = append(tags, collect(field.UIHints["tags"])...)
		tags = append(tags, collect(field.UIHints["admin.tags"])...)
	}
	return dedupe(tokensToLower(tags))
}

func fieldSection(field Field) string {
	if field.Metadata != nil {
		if candidate := strings.TrimSpace(field.Metadata[layoutSectionFieldKey]); candidate != "" {
			return candidate
		}
		if candidate := strings.TrimSpace(field.Metadata["section"]); candidate != "" {
			return candidate
		}
	}
	if field.UIHints != nil {
		if candidate := strings.TrimSpace(field.UIHints[layoutSectionFieldKey]); candidate != "" {
			return candidate
		}
		if candidate := strings.TrimSpace(field.UIHints["section"]); candidate != "" {
			return candidate
		}
	}
	return ""
}

func normaliseTokens(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		token := normaliseToken(value)
		if token == "" {
			continue
		}
		result[token] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normaliseToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func tokensToLower(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if token := normaliseToken(value); token != "" {
			out = append(out, token)
		}
	}
	return out
}

func dedupe(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func parseTokenList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if strings.HasPrefix(raw, "[") {
		var parsed []any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			tokens := make([]string, 0, len(parsed))
			for _, entry := range parsed {
				token := normaliseToken(anyToString(entry))
				if token != "" {
					tokens = append(tokens, token)
				}
			}
			return dedupe(tokens)
		}
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' })
	if len(parts) == 0 {
		if token := normaliseToken(raw); token != "" {
			return []string{token}
		}
		return nil
	}

	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		if token := normaliseToken(part); token != "" {
			tokens = append(tokens, token)
		}
	}
	return dedupe(tokens)
}

func anyToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func pruneSectionMetadata(form *FormModel, fields []Field) {
	if form == nil || len(form.Metadata) == 0 {
		return
	}

	keptSections := make(map[string]struct{})
	keptOrderKeys := make(map[string]struct{})
	for _, field := range fields {
		section := normaliseToken(fieldSection(field))
		if section == "" {
			continue
		}
		keptSections[section] = struct{}{}
		keptOrderKeys[layoutFieldOrderPrefix+section] = struct{}{}
	}

	pruneLayoutSections(form.Metadata, keptSections)
	pruneLayoutFieldOrder(form.Metadata, keptOrderKeys)
}

func pruneLayoutSections(metadata map[string]string, keptSections map[string]struct{}) {
	if len(keptSections) == 0 {
		delete(metadata, layoutSectionsKey)
		return
	}

	raw := strings.TrimSpace(metadata[layoutSectionsKey])
	if raw == "" {
		return
	}

	var sections []map[string]any
	if err := json.Unmarshal([]byte(raw), &sections); err != nil {
		return
	}

	filtered := make([]map[string]any, 0, len(sections))
	for _, section := range sections {
		id := normaliseToken(anyToString(section["id"]))
		if id == "" {
			continue
		}
		if _, ok := keptSections[id]; ok {
			filtered = append(filtered, section)
		}
	}

	if len(filtered) == 0 {
		delete(metadata, layoutSectionsKey)
		return
	}

	if payload, err := json.Marshal(filtered); err == nil {
		metadata[layoutSectionsKey] = string(payload)
	}
}

func pruneLayoutFieldOrder(metadata map[string]string, keptOrderKeys map[string]struct{}) {
	for key := range metadata {
		if !strings.HasPrefix(key, layoutFieldOrderPrefix) {
			continue
		}
		if _, ok := keptOrderKeys[key]; !ok {
			delete(metadata, key)
		}
	}
}
