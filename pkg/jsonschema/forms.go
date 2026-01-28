package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/goliatone/go-formgen/pkg/schema"
)

const defaultFormSuffix = ".edit"

// FormDiscoveryOptions configures fallback naming when no explicit forms exist.
type FormDiscoveryOptions struct {
	Slug         string
	FormIDSuffix string
}

// DiscoverFormsFromBytes parses a JSON schema document and returns form refs.
func DiscoverFormsFromBytes(raw []byte, opts FormDiscoveryOptions) ([]schema.FormRef, error) {
	if len(raw) == 0 {
		return nil, errors.New("jsonschema: raw schema is empty")
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("jsonschema: parse schema: %w", err)
	}
	return DiscoverFormsFromMap(payload, opts)
}

// DiscoverFormsFromMap derives form identifiers from a JSON schema document.
func DiscoverFormsFromMap(payload map[string]any, opts FormDiscoveryOptions) ([]schema.FormRef, error) {
	if payload == nil {
		return nil, errors.New("jsonschema: schema is nil")
	}

	if refs, ok, err := formsFromExtension(payload); err != nil {
		return nil, err
	} else if ok {
		return refs, nil
	}

	if id := strings.TrimSpace(readString(payload, "$id")); id != "" {
		return []schema.FormRef{{ID: id + resolveSuffix(opts.FormIDSuffix)}}, nil
	}

	slug := strings.TrimSpace(opts.Slug)
	if slug == "" {
		return nil, errors.New("jsonschema: slug required to derive form id")
	}
	return []schema.FormRef{{ID: slug + resolveSuffix(opts.FormIDSuffix)}}, nil
}

func formsFromExtension(payload map[string]any) ([]schema.FormRef, bool, error) {
	raw, ok := payload["x-formgen"]
	if !ok {
		return nil, false, nil
	}
	meta, ok := raw.(map[string]any)
	if !ok {
		return nil, true, errors.New("jsonschema: x-formgen must be an object")
	}

	formsRaw, ok := meta["forms"]
	if !ok {
		return nil, false, nil
	}

	list, ok := formsRaw.([]any)
	if !ok {
		return nil, true, errors.New("jsonschema: x-formgen.forms must be an array")
	}
	if len(list) == 0 {
		return nil, true, errors.New("jsonschema: x-formgen.forms is empty")
	}

	refs := make([]schema.FormRef, 0, len(list))
	for idx, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("jsonschema: x-formgen.forms[%d] must be an object", idx)
		}
		id := strings.TrimSpace(readString(entry, "id"))
		if id == "" {
			return nil, true, fmt.Errorf("jsonschema: x-formgen.forms[%d].id is required", idx)
		}
		ref := schema.FormRef{
			ID:          id,
			Title:       strings.TrimSpace(readString(entry, "title")),
			Summary:     strings.TrimSpace(readString(entry, "summary")),
			Description: strings.TrimSpace(readString(entry, "description")),
		}
		refs = append(refs, ref)
	}
	return refs, true, nil
}

func readString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func resolveSuffix(suffix string) string {
	value := strings.TrimSpace(suffix)
	if value == "" {
		return defaultFormSuffix
	}
	if strings.HasPrefix(value, ".") {
		return value
	}
	return "." + value
}
