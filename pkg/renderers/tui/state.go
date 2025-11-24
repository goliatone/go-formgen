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
	segments := strings.Split(path, ".")
	for _, segment := range segments {
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

	var (
		current     any = root
		parentMap   map[string]any
		parentSlice []any
		parentKey   string
		parentIndex = -1
	)

	for i, segment := range segments {
		last := i == len(segments)-1
		switch node := current.(type) {
		case map[string]any:
			if last {
				node[segment] = value
				return nil
			}

			nextSegment := segments[i+1]
			if idx, err := strconv.Atoi(nextSegment); err == nil {
				child, ok := node[segment].([]any)
				if !ok {
					child = make([]any, idx+1)
				} else if len(child) <= idx {
					child = append(child, make([]any, idx+1-len(child))...)
				}
				node[segment] = child
				parentMap, parentSlice, parentKey, parentIndex = node, nil, segment, -1
				if last {
					return nil
				}
				if child[idx] == nil {
					child[idx] = make(map[string]any)
				}
				current = child[idx]
				parentSlice = child
				parentIndex = idx
			} else {
				child, ok := node[segment].(map[string]any)
				if !ok || child == nil {
					child = make(map[string]any)
					node[segment] = child
				}
				parentMap, parentSlice, parentKey, parentIndex = node, nil, segment, -1
				current = child
			}

		case []any:
			idx, err := strconv.Atoi(segment)
			if err != nil {
				return fmt.Errorf("tui: expected numeric segment, got %q", segment)
			}
			if idx < 0 {
				return fmt.Errorf("tui: negative index in path %q", path)
			}

			if len(node) <= idx {
				node = append(node, make([]any, idx+1-len(node))...)
				if parentMap != nil {
					parentMap[parentKey] = node
				} else if parentSlice != nil && parentIndex >= 0 {
					parentSlice[parentIndex] = node
				}
			}

			if last {
				node[idx] = value
				if parentMap != nil {
					parentMap[parentKey] = node
				} else if parentSlice != nil && parentIndex >= 0 {
					parentSlice[parentIndex] = node
				}
				return nil
			}

			nextSegment := segments[i+1]
			if nextIdx, err := strconv.Atoi(nextSegment); err == nil {
				child, ok := node[idx].([]any)
				if !ok {
					child = []any{}
				}
				node[idx] = child
				if parentMap != nil {
					parentMap[parentKey] = node
				} else if parentSlice != nil && parentIndex >= 0 {
					parentSlice[parentIndex] = node
				}
				parentMap, parentSlice, parentKey, parentIndex = nil, node, "", idx
				current = child
			} else {
				child, ok := node[idx].(map[string]any)
				if !ok || child == nil {
					child = make(map[string]any)
				}
				node[idx] = child
				if parentMap != nil {
					parentMap[parentKey] = node
				} else if parentSlice != nil && parentIndex >= 0 {
					parentSlice[parentIndex] = node
				}
				parentMap, parentSlice, parentKey, parentIndex = nil, node, "", idx
				current = child
			}

		default:
			return fmt.Errorf("tui: unexpected container for segment %q", segment)
		}
	}

	return nil
}
