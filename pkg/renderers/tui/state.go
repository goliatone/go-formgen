package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// State tracks collected values and server-provided errors keyed by dotted
// paths. It is intentionally small; higher-level orchestration lives in the
// renderer.
type State struct {
	values map[string]any
	errors map[string][]string
}

// NewState seeds the state with prefilled values and errors.
func NewState(prefill map[string]any, errs map[string][]string) *State {
	return &State{
		values: cloneValues(prefill),
		errors: cloneErrors(errs),
	}
}

// Values returns the current value map (mutable).
func (s *State) Values() map[string]any {
	if s == nil {
		return nil
	}
	return s.values
}

// Errors returns the current errors map (mutable).
func (s *State) Errors() map[string][]string {
	if s == nil {
		return nil
	}
	return s.errors
}

// ErrorsFor returns the errors attached to a dotted path.
func (s *State) ErrorsFor(path string) []string {
	if s == nil || len(s.errors) == 0 {
		return nil
	}
	return s.errors[path]
}

// GetValue resolves a dotted path into the values map.
func (s *State) GetValue(path string) (any, bool) {
	if s == nil {
		return nil, false
	}
	return getPath(s.values, path)
}

// SetValue writes a value using a dotted path, creating intermediate maps/slices
// as needed.
func (s *State) SetValue(path string, value any) error {
	if s == nil {
		return fmt.Errorf("tui: state is nil")
	}
	if s.values == nil {
		s.values = make(map[string]any)
	}
	return setPath(s.values, path, value)
}

func cloneValues(src map[string]any) map[string]any {
	if len(src) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = deepCopy(v)
	}
	return out
}

func cloneErrors(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return make(map[string][]string)
	}
	out := make(map[string][]string, len(src))
	for k, v := range src {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func deepCopy(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(typed))
		for k, v := range typed {
			clone[k] = deepCopy(v)
		}
		return clone
	case []any:
		clone := make([]any, len(typed))
		for i, v := range typed {
			clone[i] = deepCopy(v)
		}
		return clone
	default:
		return typed
	}
}

func getPath(root map[string]any, path string) (any, bool) {
	if root == nil || path == "" {
		return nil, false
	}
	current := any(root)
	segments := strings.SplitSeq(path, ".")
	for segment := range segments {
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(segment)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			current = node[idx]
		default:
			return nil, false
		}
	}
	return current, true
}

func setPath(root map[string]any, path string, value any) error {
	if root == nil {
		return fmt.Errorf("tui: root map is nil")
	}
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return nil
	}
	_, err := setPathValue(root, segments, value, path)
	return err
}

func setPathValue(current any, segments []string, value any, fullPath string) (any, error) {
	if len(segments) == 0 {
		return value, nil
	}

	switch node := current.(type) {
	case map[string]any:
		return setMapPathValue(node, segments, value, fullPath)
	case []any:
		return setSlicePathValue(node, segments, value, fullPath)
	default:
		return nil, fmt.Errorf("tui: unexpected container for segment %q", segments[0])
	}
}

func setMapPathValue(node map[string]any, segments []string, value any, fullPath string) (any, error) {
	segment := segments[0]
	if len(segments) == 1 {
		node[segment] = value
		return node, nil
	}

	nextSegment := segments[1]
	if idx, ok := parsePathIndex(nextSegment, fullPath); ok {
		return setMapSlicePathValue(node, segment, idx, segments[2:], value, fullPath)
	}

	child := ensureMap(node[segment])
	updated, err := setPathValue(child, segments[1:], value, fullPath)
	if err != nil {
		return nil, err
	}
	node[segment] = updated
	return node, nil
}

func setMapSlicePathValue(node map[string]any, segment string, idx int, rest []string, value any, fullPath string) (any, error) {
	child := ensureSlice(node[segment], idx)
	if len(rest) == 0 {
		child[idx] = value
		node[segment] = child
		return node, nil
	}
	if child[idx] == nil {
		child[idx] = containerForNext(rest[0])
	}
	updated, err := setPathValue(child[idx], rest, value, fullPath)
	if err != nil {
		return nil, err
	}
	child[idx] = updated
	node[segment] = child
	return node, nil
}

func setSlicePathValue(node []any, segments []string, value any, fullPath string) (any, error) {
	idx, err := strconv.Atoi(segments[0])
	if err != nil {
		return nil, fmt.Errorf("tui: expected numeric segment, got %q", segments[0])
	}
	if idx < 0 {
		return nil, fmt.Errorf("tui: negative index in path %q", fullPath)
	}
	node = ensureSlice(node, idx)
	if len(segments) == 1 {
		node[idx] = value
		return node, nil
	}
	if node[idx] == nil {
		node[idx] = containerForNext(segments[1])
	}
	updated, err := setPathValue(node[idx], segments[1:], value, fullPath)
	if err != nil {
		return nil, err
	}
	node[idx] = updated
	return node, nil
}

func parsePathIndex(segment, fullPath string) (int, bool) {
	idx, err := strconv.Atoi(segment)
	if err != nil {
		return 0, false
	}
	if idx < 0 {
		return 0, false
	}
	return idx, true
}

func ensureSlice(value any, idx int) []any {
	child, ok := value.([]any)
	if !ok {
		child = []any{}
	}
	if len(child) <= idx {
		child = append(child, make([]any, idx+1-len(child))...)
	}
	return child
}

func ensureMap(value any) map[string]any {
	child, ok := value.(map[string]any)
	if !ok || child == nil {
		return make(map[string]any)
	}
	return child
}

func containerForNext(next string) any {
	if _, err := strconv.Atoi(next); err == nil {
		return []any{}
	}
	return make(map[string]any)
}
