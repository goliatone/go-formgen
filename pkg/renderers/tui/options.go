package tui

import "net/http"

// OutputFormat controls how collected values are serialized.
type OutputFormat string

const (
	// OutputFormatJSON emits application/json payloads.
	OutputFormatJSON OutputFormat = "json"
	// OutputFormatFormURLEncoded emits application/x-www-form-urlencoded payloads.
	OutputFormatFormURLEncoded OutputFormat = "form"
	// OutputFormatPrettyText emits a human-friendly text summary.
	OutputFormatPrettyText OutputFormat = "pretty"
)

// Theme captures optional formatting hints the driver can apply when printing
// messages. Keep minimal to avoid coupling renderer logic to ANSI specifics.
type Theme struct {
	PromptPrefix string
	InfoPrefix   string
	ErrorPrefix  string
}

// SubmitTransformer mutates collected values before serialization.
type SubmitTransformer func(map[string]any) (map[string]any, error)

// Option configures the TUI renderer.
type Option func(*Renderer)

// WithPromptDriver overrides the prompt driver used by the renderer.
func WithPromptDriver(driver PromptDriver) Option {
	return func(r *Renderer) {
		if driver != nil {
			r.driver = driver
		}
	}
}

// WithOutputFormat selects the output serialization format.
func WithOutputFormat(format OutputFormat) Option {
	return func(r *Renderer) {
		if format != "" {
			r.outputFormat = format
		}
	}
}

// WithHTTPClient opts into relationship option fetching using the provided
// HTTP client. When omitted, the renderer stays offline.
func WithHTTPClient(client *http.Client) Option {
	return func(r *Renderer) {
		r.httpClient = client
	}
}

// WithSubmitTransformer allows callers to mutate collected values prior to
// serialization.
func WithSubmitTransformer(fn SubmitTransformer) Option {
	return func(r *Renderer) {
		r.submitTransformer = fn
	}
}

// WithTheme applies optional message prefixes.
func WithTheme(theme Theme) Option {
	return func(r *Renderer) {
		r.theme = theme
	}
}
