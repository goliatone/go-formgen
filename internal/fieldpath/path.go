// Package fieldpath handles strict embedded-contract paths. Legacy submission
// aliases intentionally remain in the submission parser.
package fieldpath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/internal/model"
)

var grammar = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\[[0-9]+\])*(\.[A-Za-z_][A-Za-z0-9_-]*(\[[0-9]+\])*)*$`)

type Segment struct {
	Name    string
	Index   int
	IsIndex bool
}

func Parse(path string) ([]Segment, error) {
	if !grammar.MatchString(path) {
		return nil, fmt.Errorf("invalid canonical field path")
	}
	var out []Segment
	for _, part := range strings.Split(path, ".") {
		name := strings.SplitN(part, "[", 2)[0]
		if name == "__proto__" || name == "prototype" || name == "constructor" {
			return nil, fmt.Errorf("unsafe field path")
		}
		out = append(out, Segment{Name: name})
		tail := part[len(name):]
		for tail != "" {
			end := strings.IndexByte(tail, ']')
			raw := tail[1:end]
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 || strconv.Itoa(n) != raw {
				return nil, fmt.Errorf("invalid canonical index")
			}
			out = append(out, Segment{Index: n, IsIndex: true})
			tail = tail[end+1:]
		}
	}
	return out, nil
}
func Within(path, parent string) bool {
	return path == parent || strings.HasPrefix(path, parent+".") || strings.HasPrefix(path, parent+"[")
}
func Lookup(form model.FormModel, path string) (model.Field, bool) {
	resolved, ok := Resolve(form, path)
	return resolved.Field, ok
}

// Resolution retains the immediate array container for an indexed element.
// Its per-element enum and the item's own constraints are independent layers.
type Resolution struct {
	Field model.Field
	Array *model.Field
}

func Resolve(form model.FormModel, path string) (Resolution, bool) {
	segments, err := Parse(path)
	if err != nil {
		return Resolution{}, false
	}
	current := model.Field{Type: model.FieldTypeObject, Nested: form.Fields}
	var array *model.Field
	for _, segment := range segments {
		var ok bool
		array = nil
		if segment.IsIndex {
			parent := current
			array = &parent
			current, ok = arrayItem(current)
		} else {
			current, ok = namedField(current, segment.Name)
		}
		if !ok {
			return Resolution{}, false
		}
	}
	return Resolution{Field: current, Array: array}, true
}
func namedField(parent model.Field, name string) (model.Field, bool) {
	if parent.Type != model.FieldTypeObject {
		return model.Field{}, false
	}
	for _, field := range parent.Nested {
		if field.Name == name {
			return field, true
		}
	}
	return model.Field{}, false
}
func arrayItem(parent model.Field) (model.Field, bool) {
	if parent.Type != model.FieldTypeArray {
		return model.Field{}, false
	}
	if parent.Items != nil {
		return *parent.Items, true
	}
	itemType := model.FieldTypeString
	for _, value := range parent.Enum {
		switch value.(type) {
		case bool:
			itemType = model.FieldTypeBoolean
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			itemType = model.FieldTypeInteger
		case float32, float64:
			itemType = model.FieldTypeNumber
		default:
			continue
		}
		break
	}
	return model.Field{Type: itemType, Enum: parent.Enum}, true
}
func descend(value any, segment Segment) (any, bool) {
	if segment.IsIndex {
		items, ok := value.([]any)
		if !ok || segment.Index >= len(items) {
			return nil, false
		}
		return items[segment.Index], true
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	child, exists := object[segment.Name]
	return child, exists
}
func Get(values map[string]any, path string) (any, bool) {
	segments, err := Parse(path)
	if err != nil {
		return nil, false
	}
	var current any = values
	for _, segment := range segments {
		var exists bool
		current, exists = descend(current, segment)
		if !exists {
			return nil, false
		}
	}
	return current, true
}

// IndexesExist checks structural array membership, not leaf presence. Missing
// non-indexed objects may still have conditionally required descendants, but an
// item rule never creates an array member or imposes a minimum row count.
func IndexesExist(values map[string]any, path string) bool {
	segments, err := Parse(path)
	if err != nil {
		return false
	}
	lastIndex := -1
	for i, segment := range segments {
		if segment.IsIndex {
			lastIndex = i
		}
	}
	var current any = values
	for _, segment := range segments[:lastIndex+1] {
		var exists bool
		current, exists = descend(current, segment)
		if !exists {
			return false
		}
	}
	return true
}

// Remove preserves array indexes; an inactive array element becomes null.
func Remove(values map[string]any, path string) {
	segments, err := Parse(path)
	if err != nil {
		return
	}
	var parent any = values
	for _, segment := range segments[:len(segments)-1] {
		var exists bool
		parent, exists = descend(parent, segment)
		if !exists {
			return
		}
	}
	last := segments[len(segments)-1]
	if last.IsIndex {
		if items, ok := parent.([]any); ok && last.Index < len(items) {
			items[last.Index] = nil
		}
		return
	}
	if object, ok := parent.(map[string]any); ok {
		delete(object, last.Name)
	}
}
