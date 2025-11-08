package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
)

// Transformer mutates a FormModel before UI schema decorators run. Implementations
// can rename fields, inject metadata, or perform arbitrary rewrites.
type Transformer interface {
	Transform(ctx context.Context, form *model.FormModel) error
}

// TransformerFunc adapts plain functions to the Transformer interface.
type TransformerFunc func(ctx context.Context, form *model.FormModel) error

// Transform executes the wrapped function when non-nil.
func (fn TransformerFunc) Transform(ctx context.Context, form *model.FormModel) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, form)
}

// JSONPresetTransformer applies declarative overrides loaded from a JSON file.
// The document shape supports form-level metadata/UI hints and per-field patches:
//
//	{
//	  "metadata": {"layout.fieldOrder.details": "[\"id\",\"name\"]"},
//	  "uiHints": {"layout.title": "Custom"},
//	  "fields": {
//	    "title": {"label": "Custom Title", "metadata": {"layout.section": "details"}}
//	  }
//	}
type JSONPresetTransformer struct {
	document jsonTransformDocument
}

type jsonTransformDocument struct {
	Metadata map[string]string         `json:"metadata"`
	UIHints  map[string]string         `json:"uiHints"`
	Fields   map[string]jsonFieldPatch `json:"fields"`
}

type jsonFieldPatch struct {
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Placeholder string            `json:"placeholder"`
	Rename      string            `json:"rename"`
	Metadata    map[string]string `json:"metadata"`
	UIHints     map[string]string `json:"uiHints"`
}

// NewJSONPresetTransformer constructs a transformer from raw JSON bytes.
func NewJSONPresetTransformer(data []byte) (*JSONPresetTransformer, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("json preset transformer: document is empty")
	}
	var document jsonTransformDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("json preset transformer: parse document: %w", err)
	}
	return &JSONPresetTransformer{document: document}, nil
}

// NewJSONPresetTransformerFromFS loads a JSON transformer document from the
// provided filesystem path.
func NewJSONPresetTransformerFromFS(fsys fs.FS, path string) (*JSONPresetTransformer, error) {
	if fsys == nil {
		return nil, errors.New("json preset transformer: filesystem is nil")
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("json preset transformer: path is required")
	}
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("json preset transformer: read %s: %w", path, err)
	}
	return NewJSONPresetTransformer(data)
}

// Transform applies the declarative patches onto the supplied form.
func (t *JSONPresetTransformer) Transform(ctx context.Context, form *model.FormModel) error {
	if form == nil {
		return errors.New("json preset transformer: form model is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(t.document.Metadata) > 0 {
		form.Metadata = mergeStringMap(form.Metadata, t.document.Metadata)
	}
	if len(t.document.UIHints) > 0 {
		form.UIHints = mergeStringMap(form.UIHints, t.document.UIHints)
	}

	for path, patch := range t.document.Fields {
		if err := ctx.Err(); err != nil {
			return err
		}
		field := findFieldByPath(form.Fields, path)
		if field == nil {
			return fmt.Errorf("json preset transformer: field %q not found", path)
		}
		applyFieldPatch(field, patch)
	}
	return nil
}

// JavaScriptRunner defines the contract for executing user-supplied JavaScript
// against a form model. Implementations may embed an interpreter (otto, goja)
// or delegate to an external process.
type JavaScriptRunner interface {
	Run(ctx context.Context, form *model.FormModel) error
}

// JavaScriptTransformer bridges the Transformer interface with a JavaScript
// execution environment supplied by callers.
type JavaScriptTransformer struct {
	runner JavaScriptRunner
}

// NewJavaScriptTransformer wraps the provided runner. The runner is responsible
// for executing user scripts and mutating the form model.
func NewJavaScriptTransformer(runner JavaScriptRunner) *JavaScriptTransformer {
	return &JavaScriptTransformer{runner: runner}
}

// Transform delegates to the configured JavaScript runner.
func (t *JavaScriptTransformer) Transform(ctx context.Context, form *model.FormModel) error {
	if t == nil || t.runner == nil {
		return errors.New("javascript transformer: runner is nil")
	}
	if form == nil {
		return errors.New("javascript transformer: form model is nil")
	}
	return t.runner.Run(ctx, form)
}

func applyFieldPatch(field *model.Field, patch jsonFieldPatch) {
	if field == nil {
		return
	}
	if patch.Label != "" {
		field.Label = patch.Label
	}
	if patch.Description != "" {
		field.Description = patch.Description
	}
	if patch.Placeholder != "" {
		field.Placeholder = patch.Placeholder
	}
	if len(patch.Metadata) > 0 {
		field.Metadata = mergeStringMap(field.Metadata, patch.Metadata)
	}
	if len(patch.UIHints) > 0 {
		field.UIHints = mergeStringMap(field.UIHints, patch.UIHints)
	}
	if strings.TrimSpace(patch.Rename) != "" {
		field.Name = strings.TrimSpace(patch.Rename)
	}
}

func findFieldByPath(fields []model.Field, path string) *model.Field {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	segments := strings.Split(path, ".")
	return walkFieldsByPath(fields, segments)
}

func walkFieldsByPath(fields []model.Field, segments []string) *model.Field {
	if len(segments) == 0 {
		return nil
	}
	head := segments[0]
	for idx := range fields {
		field := &fields[idx]
		if field.Name != head {
			continue
		}
		if len(segments) == 1 {
			return field
		}
		if segments[1] == "items" {
			return descendArray(field, segments[2:])
		}
		return walkFieldsByPath(field.Nested, segments[1:])
	}
	return nil
}

func descendArray(field *model.Field, segments []string) *model.Field {
	if field == nil {
		return nil
	}
	if field.Items == nil {
		return nil
	}
	if len(segments) == 0 {
		return field.Items
	}
	if segments[0] == "items" {
		return descendArray(field.Items, segments[1:])
	}
	return walkFieldsByPath(field.Items.Nested, segments)
}

func mergeStringMap(dst, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
