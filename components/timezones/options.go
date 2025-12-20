package timezones

import "net/http"

type EmptySearchMode string

const (
	EmptySearchNone EmptySearchMode = "none"
	EmptySearchTop  EmptySearchMode = "top"
)

type GuardFunc func(r *http.Request) error

type Options struct {
	RoutePath       string
	SearchParam     string
	LimitParam      string
	DefaultLimit    int
	MaxLimit        int
	EmptySearchMode EmptySearchMode
	Guard           GuardFunc

	Zones []string
}

type OptionFn func(*Options)

func DefaultOptions() Options {
	return Options{
		RoutePath:       "/api/timezones",
		SearchParam:     "q",
		LimitParam:      "limit",
		DefaultLimit:    50,
		MaxLimit:        200,
		EmptySearchMode: EmptySearchNone,
	}
}

func NewOptions(fns ...OptionFn) Options {
	opts := DefaultOptions()
	for _, fn := range fns {
		if fn == nil {
			continue
		}
		fn(&opts)
	}
	if opts.DefaultLimit <= 0 {
		opts.DefaultLimit = 50
	}
	if opts.MaxLimit <= 0 {
		opts.MaxLimit = 200
	}
	if opts.EmptySearchMode == "" {
		opts.EmptySearchMode = EmptySearchNone
	}
	if opts.RoutePath == "" {
		opts.RoutePath = "/api/timezones"
	}
	if opts.SearchParam == "" {
		opts.SearchParam = "q"
	}
	if opts.LimitParam == "" {
		opts.LimitParam = "limit"
	}
	if opts.Zones != nil {
		opts.Zones = append([]string{}, opts.Zones...)
	}
	return opts
}

func WithRoutePath(path string) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.RoutePath = path
	}
}

func WithSearchParam(name string) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.SearchParam = name
	}
}

func WithLimitParam(name string) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.LimitParam = name
	}
}

func WithDefaultLimit(limit int) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.DefaultLimit = limit
	}
}

func WithMaxLimit(limit int) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.MaxLimit = limit
	}
}

func WithEmptySearchMode(mode EmptySearchMode) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.EmptySearchMode = mode
	}
}

func WithGuard(guard GuardFunc) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		o.Guard = guard
	}
}

func WithZones(zones []string) OptionFn {
	return func(o *Options) {
		if o == nil {
			return
		}
		if zones == nil {
			o.Zones = nil
			return
		}
		o.Zones = append([]string{}, zones...)
	}
}

func clampLimit(limit int, opts Options) int {
	if limit < 0 {
		return 0
	}
	if limit == 0 {
		limit = opts.DefaultLimit
	}
	if opts.MaxLimit > 0 && limit > opts.MaxLimit {
		return opts.MaxLimit
	}
	return limit
}
