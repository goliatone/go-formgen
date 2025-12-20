package timezones

import "net/http"

// Component is a small, extraction-friendly wrapper around the timezone handler,
// its configuration, and routing helpers.
type Component struct {
	opts Options
}

// New constructs a new component with default options plus any overrides.
func New(fns ...OptionFn) *Component {
	opts := NewOptions(fns...)
	return &Component{opts: opts}
}

// Options returns a copy of the component configuration.
func (c *Component) Options() Options {
	if c == nil {
		return DefaultOptions()
	}
	return NewOptions(func(o *Options) { *o = c.opts })
}

// Handler returns a net/http handler for timezone queries.
func (c *Component) Handler() http.Handler {
	if c == nil {
		return Handler()
	}
	return HandlerWithOptions(c.opts)
}

// RegisterRoutes registers the component handler under basePath on mux.
func (c *Component) RegisterRoutes(mux Mux, basePath string) (string, error) {
	if c == nil {
		return RegisterRoutes(mux, basePath)
	}
	return RegisterRoutesWithOptions(mux, basePath, c.opts)
}
