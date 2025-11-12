package render

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
}
