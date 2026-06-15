package submission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

// ParseRequest parses a submitted HTTP request using its content type.
func ParseRequest(form model.FormModel, req *http.Request, options ...Option) (Result, error) {
	if req == nil {
		return Result{}, fmt.Errorf("submission: request is nil")
	}
	contentType, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	switch strings.ToLower(contentType) {
	case "application/json":
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return Result{}, err
		}
		return ParseJSON(form, body, options...)
	case "multipart/form-data":
		cfg := applyOptions(options)
		req.Body = http.MaxBytesReader(nil, req.Body, cfg.MaxBodyBytes)
		// #nosec G120 -- MaxBytesReader caps the total multipart body before parsing.
		if err := req.ParseMultipartForm(cfg.MaxMemory); err != nil {
			return Result{}, err
		}
		values := url.Values{}
		if req.MultipartForm != nil {
			for key, list := range req.MultipartForm.Value {
				values[key] = append([]string(nil), list...)
			}
		}
		return ParseValues(form, values, options...), nil
	case "application/x-www-form-urlencoded", "":
		if err := req.ParseForm(); err != nil {
			return Result{}, err
		}
		return ParseValues(form, req.PostForm, options...), nil
	default:
		if strings.HasSuffix(strings.ToLower(contentType), "+json") {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return Result{}, err
			}
			return ParseJSON(form, body, options...)
		}
		return Result{}, fmt.Errorf("submission: unsupported content type %q", contentType)
	}
}

// ParseJSON parses an application/json body into submitted Values.
func ParseJSON(form model.FormModel, body []byte, options ...Option) (Result, error) {
	var payload any
	if err := strictDecodeJSON(bytes.NewReader(body), &payload); err != nil {
		return Result{
			Values: Values{},
			Issues: []Issue{issue(CodeInvalidJSON, "", "invalid JSON body", nil)},
		}, nil
	}
	obj, ok := payload.(map[string]any)
	if !ok {
		return Result{
			Values: Values{},
			Issues: []Issue{issue(CodeType, "", "JSON body must be an object", payload)},
		}, nil
	}
	return ParseMap(form, obj, options...), nil
}

// ParseValues parses form-urlencoded or multipart values into submitted Values.
func ParseValues(form model.FormModel, values url.Values, options ...Option) Result {
	cfg := applyOptions(options)
	idx := newFieldIndex(form)
	result := Result{Values: Values{}}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		list := values[key]
		if len(list) == 0 {
			list = []string{""}
		}
		segments := parsePath(key)
		if len(segments) == 0 {
			continue
		}
		field, known := idx.fieldFor(segments)
		if !known {
			result.handleUnknown(cfg, key, unknownValue(list))
			continue
		}
		repeatedAsArray := shouldTreatRepeatedKeyAsArray(field, segments, len(list) > 1)
		for i, raw := range list {
			if len(list) > 1 && !repeatedAsArray && i > 0 {
				result.Issues = append(result.Issues, issue(CodePathConflict, canonicalPath(segments), "duplicate value for scalar field", raw))
				continue
			}
			path := segmentsForRepeatedValue(segments, repeatedAsArray)
			if pathField, ok := idx.fieldFor(path); ok {
				field = pathField
			}
			coerced, issues := CoerceValue(field, raw, cfg, canonicalPath(path))
			result.Issues = append(result.Issues, issues...)
			if errIssue := setValue(result.Values, path, coerced); errIssue != nil {
				result.Issues = append(result.Issues, *errIssue)
			}
		}
	}
	return result
}

// ParseMap parses a generic map into submitted Values.
func ParseMap(form model.FormModel, values map[string]any, options ...Option) Result {
	cfg := applyOptions(options)
	idx := newFieldIndex(form)
	result := Result{Values: Values{}}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		raw := values[key]
		segments := parsePath(key)
		if len(segments) == 0 {
			continue
		}
		field, known := idx.fieldFor(segments)
		if !known {
			result.handleUnknown(cfg, key, raw)
			continue
		}
		coerced, issues := CoerceValue(field, raw, cfg, canonicalPath(segments))
		result.Issues = append(result.Issues, issues...)
		if errIssue := setValue(result.Values, segments, coerced); errIssue != nil {
			result.Issues = append(result.Issues, *errIssue)
		}
	}
	return result
}

func (r *Result) handleUnknown(cfg Options, key string, value any) {
	switch cfg.UnknownFields {
	case UnknownIgnore:
		return
	case UnknownPreserve:
		if errIssue := setValue(r.Values, parsePath(key), value); errIssue != nil {
			r.Issues = append(r.Issues, *errIssue)
		}
	default:
		r.Issues = append(r.Issues, issue(CodeUnknownField, key, fmt.Sprintf("unknown field %q", key), value))
	}
}

func unknownValue(values []string) any {
	if len(values) == 1 {
		return values[0]
	}
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func shouldTreatRepeatedKeyAsArray(field model.Field, segments []pathSegment, repeated bool) bool {
	if !repeated {
		return false
	}
	if len(segments) == 0 {
		return false
	}
	last := segments[len(segments)-1]
	if last.Append || last.Index != nil {
		return true
	}
	return field.Type == model.FieldTypeArray
}

func segmentsForRepeatedValue(segments []pathSegment, repeated bool) []pathSegment {
	out := append([]pathSegment(nil), segments...)
	if !repeated || len(out) == 0 {
		return out
	}
	last := out[len(out)-1]
	if last.Append || last.Index != nil {
		return out
	}
	out = append(out, pathSegment{Append: true})
	return out
}

func setValue(root map[string]any, segments []pathSegment, value any) *Issue {
	if len(segments) == 0 {
		return nil
	}
	path := canonicalPath(segments)
	if segments[0].Name == "" {
		return &[]Issue{issue(CodePathConflict, path, "path must start with a field name", value)}[0]
	}
	current := any(root)
	for i, segment := range segments {
		last := i == len(segments)-1
		switch node := current.(type) {
		case map[string]any:
			if segment.Name == "" {
				return &[]Issue{issue(CodePathConflict, path, "array index cannot target an object", value)}[0]
			}
			if last {
				return assignMapValue(node, segment.Name, value, path)
			}
			next, ok := node[segment.Name]
			if !ok {
				next = newContainerFor(segments[i+1])
				node[segment.Name] = next
			}
			current = next
		case []any:
			idx, appendMode, ok := segmentArrayIndex(segment, len(node))
			if !ok {
				return &[]Issue{issue(CodePathConflict, path, "object field cannot target an array", value)}[0]
			}
			if appendMode {
				node = append(node, nil)
				idx = len(node) - 1
				replaceArrayAt(root, segments[:i], node)
			}
			for len(node) <= idx {
				node = append(node, nil)
				replaceArrayAt(root, segments[:i], node)
			}
			if last {
				if node[idx] != nil {
					return &[]Issue{issue(CodePathConflict, path, "duplicate value for path", value)}[0]
				}
				node[idx] = value
				replaceArrayAt(root, segments[:i], node)
				return nil
			}
			if node[idx] == nil {
				node[idx] = newContainerFor(segments[i+1])
				replaceArrayAt(root, segments[:i], node)
			}
			current = node[idx]
		default:
			return &[]Issue{issue(CodePathConflict, path, "path collides with scalar value", value)}[0]
		}
	}
	return nil
}

func assignMapValue(node map[string]any, key string, value any, path string) *Issue {
	existing, ok := node[key]
	if !ok {
		node[key] = value
		return nil
	}
	switch current := existing.(type) {
	case []any:
		node[key] = append(current, value)
		return nil
	default:
		node[key] = []any{current, value}
		return nil
	}
}

func strictDecodeJSON(reader io.Reader, dest any) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("json contains trailing content")
	}
	return nil
}

func newContainerFor(next pathSegment) any {
	if next.Index != nil || next.Append {
		return []any{}
	}
	return map[string]any{}
}

func segmentArrayIndex(segment pathSegment, length int) (int, bool, bool) {
	if segment.Append {
		return length, true, true
	}
	if segment.Index != nil {
		if *segment.Index < 0 {
			return 0, false, false
		}
		return *segment.Index, false, true
	}
	return 0, false, false
}

func replaceArrayAt(root map[string]any, prefix []pathSegment, value []any) {
	if len(prefix) == 0 {
		return
	}
	if len(prefix) == 1 && prefix[0].Name != "" {
		root[prefix[0].Name] = value
		return
	}
	var current any = root
	for i, segment := range prefix {
		last := i == len(prefix)-1
		switch node := current.(type) {
		case map[string]any:
			if last {
				node[segment.Name] = value
				return
			}
			current = node[segment.Name]
		case []any:
			idx := 0
			if segment.Index != nil {
				idx = *segment.Index
			}
			if idx >= 0 && idx < len(node) {
				if last {
					node[idx] = value
					return
				}
				current = node[idx]
			}
		}
	}
}
