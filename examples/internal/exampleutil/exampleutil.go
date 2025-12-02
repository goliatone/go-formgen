package exampleutil

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
)

// FixturePath returns the absolute path to a fixture located under
// examples/fixtures.
func FixturePath(name string) string {
	root := examplesRoot()
	return filepath.Join(root, "fixtures", name)
}

// RuntimeAssetsPath returns the absolute path to the compiled relationship
// runtime bundle under client/dist/browser.
func RuntimeAssetsPath() (string, error) {
	root := examplesRoot()
	path := filepath.Join(root, "..", "client", "dist", "browser")
	if _, err := os.Stat(path); err != nil {
		return filepath.Clean(path), err
	}
	return filepath.Clean(path), nil
}

// ResolveSource converts the raw input into an OpenAPI source and a canonical
// cache key. Accepts filesystem paths or HTTP(S) URLs.
func ResolveSource(raw string) (pkgopenapi.Source, string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return nil, "", errors.New("exampleutil: source is required")
	}

	if isURL(target) {
		parsed, err := url.ParseRequestURI(target)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, "", fmt.Errorf("exampleutil: invalid URL %q", target)
		}
		return pkgopenapi.SourceFromURL(parsed.String()), parsed.String(), nil
	}

	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, "", fmt.Errorf("exampleutil: resolve path %q: %w", target, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, "", fmt.Errorf("exampleutil: stat %q: %w", abs, err)
	}
	return pkgopenapi.SourceFromFile(abs), abs, nil
}

func isURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func examplesRoot() string {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		panic("exampleutil: unable to determine file location")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
}
