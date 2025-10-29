package render

// Registry scaffolding follows go-form-gen.md:137-159 and go-form-gen.md:223-239.

import (
	"fmt"
	"sort"
	"sync"
)

// Registry stores renderers by name, providing discovery and duplication
// safeguards. Implementations can embed or wrap this for dependency injection.
type Registry struct {
	mu        sync.RWMutex
	renderers map[string]Renderer
}

// NewRegistry creates an empty registry instance.
func NewRegistry() *Registry {
	return &Registry{
		renderers: make(map[string]Renderer),
	}
}

// Register adds a renderer by its Name(). Duplicate names return an error.
func (r *Registry) Register(renderer Renderer) error {
	if renderer == nil {
		return fmt.Errorf("render: renderer is required")
	}
	name := renderer.Name()
	if name == "" {
		return fmt.Errorf("render: renderer name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.renderers[name]; exists {
		return fmt.Errorf("render: renderer %q already registered", name)
	}

	r.renderers[name] = renderer
	return nil
}

// MustRegister panics on registration failure. Useful for init-time wiring.
func (r *Registry) MustRegister(renderer Renderer) {
	if err := r.Register(renderer); err != nil {
		panic(err)
	}
}

// Get retrieves a renderer by name.
func (r *Registry) Get(name string) (Renderer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	renderer, ok := r.renderers[name]
	if !ok {
		return nil, fmt.Errorf("render: renderer %q not found", name)
	}
	return renderer, nil
}

// MustGet panics if the renderer is missing.
func (r *Registry) MustGet(name string) Renderer {
	renderer, err := r.Get(name)
	if err != nil {
		panic(err)
	}
	return renderer
}

// List returns a sorted list of renderer names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.renderers))
	for name := range r.renderers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has reports whether a renderer is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.renderers[name]
	return ok
}
