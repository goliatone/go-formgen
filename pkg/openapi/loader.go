package openapi

// Loader contracts mirror the README flow described in go-form-gen.md:304-331.

import (
	"context"
	"io/fs"
	"net/http"
	"time"
)

// Loader fetches OpenAPI documents from different sources (filesystem, fs.FS,
// HTTP). Implementations live under internal/openapi but satisfy this contract.
type Loader interface {
	Load(ctx context.Context, src Source) (Document, error)
}

// LoaderOptions configures how a Loader resolves sources. It collects the knobs
// referenced throughout go-form-gen.md:25-77 (offline-first with optional HTTP).
type LoaderOptions struct {
	// FileSystem enables loading from an abstract filesystem; defaults to the
	// operating system if nil.
	FileSystem fs.FS

	// HTTPClient allows callers to inject custom HTTP behaviour (timeouts,
	// proxies). Nil means HTTP sources are disabled unless AllowHTTPFallback is
	// true.
	HTTPClient *http.Client

	// AllowHTTPFallback toggles the default HTTP loader using http.DefaultClient
	// when no client is supplied. Keeping this explicit preserves offline-first
	// behaviour promised in the README.
	AllowHTTPFallback bool

	// RequestTimeout caps remote fetch durations when AllowHTTPFallback is true.
	RequestTimeout time.Duration
}

// LoaderOption mutates LoaderOptions prior to construction.
type LoaderOption func(*LoaderOptions)

// WithFileSystem injects an fs.FS implementation for relative paths.
func WithFileSystem(files fs.FS) LoaderOption {
	return func(opts *LoaderOptions) {
		opts.FileSystem = files
	}
}

// WithHTTPClient injects a custom HTTP client for remote OpenAPI documents.
func WithHTTPClient(client *http.Client) LoaderOption {
	return func(opts *LoaderOptions) {
		opts.HTTPClient = client
	}
}

// WithHTTPFallback enables HTTP loading using http.DefaultClient and assigns an
// optional timeout.
func WithHTTPFallback(timeout time.Duration) LoaderOption {
	return func(opts *LoaderOptions) {
		opts.AllowHTTPFallback = true
		opts.RequestTimeout = timeout
	}
}

// WithDefaultSources enables the built-in HTTP loader using the default client
// when no explicit client is provided. This mirrors the Quick Start examples in
// the README.
func WithDefaultSources() LoaderOption {
	return func(opts *LoaderOptions) {
		if !opts.AllowHTTPFallback && opts.HTTPClient == nil {
			opts.AllowHTTPFallback = true
		}
	}
}

// NewLoaderOptions applies a set of LoaderOption values and returns the
// resulting configuration. Implementations can embed this helper to stay
// consistent.
func NewLoaderOptions(options ...LoaderOption) LoaderOptions {
	cfg := LoaderOptions{}
	for _, opt := range options {
		opt(&cfg)
	}
	return cfg
}

// Construction helpers live in the top-level formgen package to prevent import cycles.
