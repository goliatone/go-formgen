package validation

import (
	"context"
	"errors"
	"strings"

	pkgjsonschema "github.com/goliatone/go-formgen/pkg/jsonschema"
	"github.com/goliatone/go-formgen/pkg/schema"
)

// SchemaIssue represents a validation error with optional location metadata.
type SchemaIssue struct {
	Path    string `json:"path,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// SchemaValidationResult captures validation outcomes for builder previews.
type SchemaValidationResult struct {
	Valid  bool          `json:"valid"`
	Issues []SchemaIssue `json:"issues,omitempty"`
}

// JSONSchemaValidationOptions configures validation behaviour.
type JSONSchemaValidationOptions struct {
	Loader          pkgjsonschema.Loader
	ResolverOptions pkgjsonschema.ResolveOptions
	Normalize       schema.NormalizeOptions
}

// ValidateJSONSchema checks a Draft 2020-12 schema for adapter compatibility.
func ValidateJSONSchema(ctx context.Context, src schema.Source, raw []byte, opts JSONSchemaValidationOptions) SchemaValidationResult {
	result := SchemaValidationResult{Valid: true}
	if src == nil {
		src = pkgjsonschema.SourceFromFS("schema.json")
	}

	doc, err := schema.NewDocument(src, raw)
	if err != nil {
		result.Valid = false
		result.Issues = []SchemaIssue{issueFromError(err)}
		return result
	}

	loader := opts.Loader
	if loader == nil {
		loader = failLoader{}
	}

	adapter := pkgjsonschema.NewAdapter(loader, pkgjsonschema.WithResolverOptions(opts.ResolverOptions))
	if _, err := adapter.Normalize(ctx, doc, opts.Normalize); err != nil {
		result.Valid = false
		result.Issues = []SchemaIssue{issueFromError(err)}
		return result
	}

	return result
}

type failLoader struct{}

func (failLoader) Load(_ context.Context, _ pkgjsonschema.Source) (schema.Document, error) {
	return schema.Document{}, errors.New("jsonschema validation: loader is not configured")
}

func issueFromError(err error) SchemaIssue {
	if err == nil {
		return SchemaIssue{Message: "unknown error"}
	}
	var overlayErr pkgjsonschema.OverlayError
	if errors.As(err, &overlayErr) {
		return SchemaIssue{
			Path:    overlayErr.Path,
			Field:   fieldPathFromPointer(overlayErr.Path),
			Message: strings.TrimSpace(overlayErr.Message),
		}
	}

	msg := strings.TrimSpace(err.Error())
	path := extractJSONPointer(msg)
	if path != "" {
		msg = strings.Replace(msg, " at "+path, "", 1)
	}
	msg = strings.TrimPrefix(msg, "jsonschema: ")
	msg = strings.TrimPrefix(msg, "jsonschema resolver: ")
	msg = strings.TrimPrefix(msg, "jsonschema overlay: ")
	msg = strings.TrimSpace(msg)

	return SchemaIssue{
		Path:    path,
		Field:   fieldPathFromPointer(path),
		Message: msg,
	}
}

func extractJSONPointer(message string) string {
	if message == "" {
		return ""
	}
	if idx := strings.LastIndex(message, " at "); idx >= 0 {
		candidate := strings.TrimSpace(message[idx+4:])
		return trimPointer(candidate)
	}
	if idx := strings.LastIndex(message, "#/"); idx >= 0 {
		candidate := strings.TrimSpace(message[idx:])
		return trimPointer(candidate)
	}
	return ""
}

func trimPointer(pointer string) string {
	if pointer == "" {
		return ""
	}
	trimmed := strings.TrimRight(pointer, ".)];,")
	return strings.TrimSpace(trimmed)
}

func fieldPathFromPointer(pointer string) string {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "#")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return ""
	}

	parts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(parts))
	for idx := 0; idx < len(parts); idx++ {
		segment := strings.ReplaceAll(parts[idx], "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		switch segment {
		case "properties":
			if idx+1 < len(parts) {
				next := strings.ReplaceAll(parts[idx+1], "~1", "/")
				next = strings.ReplaceAll(next, "~0", "~")
				out = append(out, next)
				idx++
			}
		case "items":
			out = append(out, "items")
		case "oneOf", "anyOf", "allOf":
			if idx+1 < len(parts) && isNumeric(parts[idx+1]) {
				idx++
			}
		case "$defs":
			if idx+1 < len(parts) {
				idx++
			}
		default:
			if segment == "" {
				continue
			}
			out = append(out, segment)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, ".")
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
