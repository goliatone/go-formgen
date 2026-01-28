package orchestrator

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/goliatone/go-formgen/pkg/schema"
)

// FormatAdapter aliases the canonical adapter interface for convenience.
type FormatAdapter = schema.FormatAdapter

// AdapterRegistry stores format adapters by name.
type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]schema.FormatAdapter
}

// NewAdapterRegistry creates an empty adapter registry.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[string]schema.FormatAdapter),
	}
}

// Register adds an adapter by its Name(). Duplicate names return an error.
func (r *AdapterRegistry) Register(adapter schema.FormatAdapter) error {
	if adapter == nil {
		return fmt.Errorf("orchestrator: adapter is required")
	}
	name := normalizeAdapterName(adapter.Name())
	if name == "" {
		return fmt.Errorf("orchestrator: adapter name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("orchestrator: adapter %q already registered", name)
	}

	r.adapters[name] = adapter
	return nil
}

// MustRegister panics on registration failure.
func (r *AdapterRegistry) MustRegister(adapter schema.FormatAdapter) {
	if err := r.Register(adapter); err != nil {
		panic(err)
	}
}

// Get retrieves an adapter by name.
func (r *AdapterRegistry) Get(name string) (schema.FormatAdapter, error) {
	key := normalizeAdapterName(name)
	if key == "" {
		return nil, fmt.Errorf("orchestrator: adapter name is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[key]
	if !ok {
		return nil, fmt.Errorf("orchestrator: adapter %q not found", key)
	}
	return adapter, nil
}

// List returns a sorted list of adapter names.
func (r *AdapterRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has reports whether an adapter is registered.
func (r *AdapterRegistry) Has(name string) bool {
	key := normalizeAdapterName(name)
	if key == "" {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.adapters[key]
	return ok
}

// Detect returns all adapters that match the provided source payload.
func (r *AdapterRegistry) Detect(src schema.Source, raw []byte) []schema.FormatAdapter {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.adapters) == 0 {
		return nil
	}

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)

	var matches []schema.FormatAdapter
	for _, name := range names {
		adapter := r.adapters[name]
		if adapter == nil {
			continue
		}
		if adapter.Detect(src, raw) {
			matches = append(matches, adapter)
		}
	}
	return matches
}

func normalizeAdapterName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
