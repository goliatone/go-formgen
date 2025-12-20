package timezones

import (
	"fmt"
	"net/http"
	"strings"
)

// Mux is the minimal interface required to register a net/http handler.
// It is satisfied by *http.ServeMux.
type Mux interface {
	Handle(pattern string, handler http.Handler)
}

// MountPath returns the full mount path for the component route under basePath.
func MountPath(basePath string, fns ...OptionFn) string {
	opts := NewOptions(fns...)
	return mountPath(basePath, opts.RoutePath)
}

// RegisterRoutes registers the timezone handler under basePath on mux.
func RegisterRoutes(mux Mux, basePath string, fns ...OptionFn) (string, error) {
	opts := NewOptions(fns...)
	return RegisterRoutesWithOptions(mux, basePath, opts)
}

// RegisterRoutesWithOptions registers a handler under basePath using a pre-built Options value.
// Callers are expected to pass an Options value produced by NewOptions (or equivalent) so defaults apply.
func RegisterRoutesWithOptions(mux Mux, basePath string, opts Options) (string, error) {
	if mux == nil {
		return "", fmt.Errorf("timezones: missing mux")
	}
	opts = NewOptions(func(o *Options) { *o = opts })
	pattern := mountPath(basePath, opts.RoutePath)
	mux.Handle(pattern, HandlerWithOptions(opts))
	return pattern, nil
}

func mountPath(basePath, routePath string) string {
	basePath = strings.TrimSpace(basePath)
	routePath = strings.TrimSpace(routePath)

	if routePath == "" {
		routePath = "/"
	}
	if !strings.HasPrefix(routePath, "/") {
		routePath = "/" + routePath
	}

	if basePath == "" || basePath == "/" {
		return routePath
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimRight(basePath, "/")
	return basePath + routePath
}
