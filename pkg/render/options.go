package render

import (
	"github.com/goliatone/formgen/pkg/visibility"
	"github.com/goliatone/go-theme"
)

// RenderOptions describe per-request data that renderers can use to customise
// their output without mutating the form model pipeline.
type RenderOptions struct {
	// Method overrides the HTTP method declared by the form model. Renderers are
	// responsible for translating unsupported verbs (PATCH/PUT/DELETE) into
	// browser-friendly POST submissions plus a hidden _method input when needed.
	Method string
	// Values pre-populates rendered controls using dotted field paths (e.g.
	// "author.email"). Renderers can decide how to handle nested values or
	// collections for advanced components such as chips/typeahead controls.
	Values map[string]any
	// Errors surfaces server-side validation feedback keyed by field path. The
	// vanilla renderer maps these into inline chrome plus data-validation
	// attributes so the runtime and assistive tech can reflect the state without
	// waiting for client-side validation.
	Errors map[string][]string
	// Theme passes renderer configuration derived from a go-theme Selection so
	// renderers can resolve partials, assets, and tokens consistently.
	Theme *theme.RendererConfig
	// VisibilityContext carries evaluator-specific inputs such as current form
	// values or feature flags used to decide whether a field should render.
	VisibilityContext visibility.Context
	// TopPadding controls how many leading newlines renderers emit before the
	// root form markup when no external stylesheets or inline styles are
	// present. A zero value allows the renderer to apply its default.
	TopPadding int
}
