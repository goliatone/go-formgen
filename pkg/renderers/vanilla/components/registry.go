package components

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/goliatone/formgen/pkg/model"
	rendertemplate "github.com/goliatone/formgen/pkg/render/template"
)

// Renderer defines the contract component renderers must satisfy. Implementations
// receive the component-specific field and can write HTML into buf using the
// supplied template renderer or custom logic.
type Renderer func(buf *bytes.Buffer, field model.Field, data ComponentData) error

// ComponentData carries helpers and configuration for component renderers.
type ComponentData struct {
	Template    rendertemplate.TemplateRenderer
	RenderChild func(value any) (string, error)
	Config      map[string]any
}

// Script describes JavaScript dependencies a component needs to emit once per
// render.
type Script struct {
	Src    string
	Type   string
	Inline string
	Async  bool
	Defer  bool
	Module bool
	Attrs  map[string]string
}

// Descriptor bundles the renderer implementation with any asset dependencies.
type Descriptor struct {
	Name        string
	Renderer    Renderer
	Stylesheets []string
	Scripts     []Script
}

// Registry tracks component descriptors keyed by name. Callers can register new
// components or override defaults.
type Registry struct {
	mu         sync.RWMutex
	components map[string]Descriptor
}

// New creates an empty registry.
func New() *Registry {
	return &Registry{
		components: make(map[string]Descriptor),
	}
}

// Clone returns a deep copy of the registry to allow isolated mutations.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cloned := New()
	for name, descriptor := range r.components {
		cloned.components[name] = cloneDescriptor(descriptor)
	}
	return cloned
}

// Register associates a descriptor with the provided name. Existing entries are
// replaced.
func (r *Registry) Register(name string, descriptor Descriptor) error {
	if name = normalize(name); name == "" {
		return fmt.Errorf("components: component name is required")
	}
	if descriptor.Renderer == nil {
		return fmt.Errorf("components: renderer for %q is nil", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	descriptor.Name = name
	r.components[name] = cloneDescriptor(descriptor)
	return nil
}

// MustRegister mirrors Register but panics on error, simplifying default registry
// setup.
func (r *Registry) MustRegister(name string, descriptor Descriptor) {
	if err := r.Register(name, descriptor); err != nil {
		panic(err)
	}
}

// Descriptor fetches a descriptor by name.
func (r *Registry) Descriptor(name string) (Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.components[normalize(name)]
	if !ok {
		return Descriptor{}, false
	}
	return cloneDescriptor(descriptor), true
}

// Names returns a sorted slice of registered component names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.components))
	for name := range r.components {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Assets resolves dependency aggregates for the provided component names.
func (r *Registry) Assets(names []string) (stylesheets []string, scripts []Script) {
	if len(names) == 0 {
		return nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	seenStyles := make(map[string]struct{})
	seenScripts := make(map[string]struct{})

	for _, name := range names {
		descriptor, ok := r.components[normalize(name)]
		if !ok {
			continue
		}
		for _, href := range descriptor.Stylesheets {
			if href == "" {
				continue
			}
			if _, exists := seenStyles[href]; exists {
				continue
			}
			seenStyles[href] = struct{}{}
			stylesheets = append(stylesheets, href)
		}
		for _, script := range descriptor.Scripts {
			key := scriptKey(script)
			if _, exists := seenScripts[key]; exists {
				continue
			}
			seenScripts[key] = struct{}{}
			scripts = append(scripts, script)
		}
	}
	return stylesheets, scripts
}

func cloneDescriptor(src Descriptor) Descriptor {
	clone := Descriptor{
		Name:        src.Name,
		Renderer:    src.Renderer,
		Stylesheets: slices.Clone(src.Stylesheets),
		Scripts:     make([]Script, len(src.Scripts)),
	}
	for idx, script := range src.Scripts {
		clone.Scripts[idx] = Script{
			Src:    script.Src,
			Type:   script.Type,
			Inline: script.Inline,
			Async:  script.Async,
			Defer:  script.Defer,
			Module: script.Module,
			Attrs:  cloneStringMap(script.Attrs),
		}
	}
	return clone
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func scriptKey(script Script) string {
	if script.Src != "" {
		return "src:" + script.Src
	}
	return "inline:" + script.Inline
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
