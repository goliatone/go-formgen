package gotemplate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/flosch/pongo2/v6"
	gotemplatepkg "github.com/goliatone/go-template"

	"github.com/goliatone/go-formgen/pkg/render/template"
)

// Option configures the go-template adapter before construction.
type Option func(*config)

type config struct {
	baseDir    string
	templates  fs.FS
	extension  string
	templateFn map[string]any
	globalData map[string]any
}

// WithBaseDir configures the underlying engine to load templates from a base
// directory on disk.
func WithBaseDir(dir string) Option {
	return func(cfg *config) {
		cfg.baseDir = strings.TrimSpace(dir)
	}
}

// WithFS configures the underlying engine to load templates from an fs.FS.
func WithFS(files fs.FS) Option {
	return func(cfg *config) {
		cfg.templates = files
	}
}

// WithExtension overrides the default template extension used by the engine.
func WithExtension(ext string) Option {
	return func(cfg *config) {
		trimmed := strings.TrimSpace(ext)
		if trimmed == "" {
			return
		}
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		cfg.extension = trimmed
	}
}

// WithTemplateFunc registers helper functions or filters when the engine loads.
func WithTemplateFunc(funcs map[string]any) Option {
	return func(cfg *config) {
		if len(funcs) == 0 {
			return
		}
		if cfg.templateFn == nil {
			cfg.templateFn = make(map[string]any, len(funcs))
		}
		for name, fn := range funcs {
			cfg.templateFn[strings.TrimSpace(name)] = fn
		}
	}
}

// WithGlobalData seeds global context values available to every template.
func WithGlobalData(data map[string]any) Option {
	return func(cfg *config) {
		if len(data) == 0 {
			return
		}
		if cfg.globalData == nil {
			cfg.globalData = make(map[string]any, len(data))
		}
		for key, value := range data {
			cfg.globalData[strings.TrimSpace(key)] = value
		}
	}
}

// WithGoTemplateOptions exists for backward compatibility with earlier versions
// of this adapter but is currently a no-op.
func WithGoTemplateOptions(_ ...gotemplatepkg.Option) Option {
	return func(*config) {}
}

// Engine satisfies the template.TemplateRenderer contract defined in
// go-form-gen.md:443-460 using a pongo2-backed template set.
type Engine struct {
	mu sync.RWMutex

	templateSet *pongo2.TemplateSet
	templates   map[string]*pongo2.Template
	tplExt      string
}

// Ensure Engine implements the TemplateRenderer interface.
var _ template.TemplateRenderer = (*Engine)(nil)

// New constructs an Engine using the provided configuration options.
func New(options ...Option) (*Engine, error) {
	cfg := &config{
		extension: ".tpl",
	}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(cfg)
	}

	if cfg.baseDir == "" && cfg.templates == nil {
		return nil, errors.New("gotemplate: need to provide either base dir or fs.FS")
	}

	var loaders []pongo2.TemplateLoader
	if cfg.baseDir != "" {
		loader, err := pongo2.NewLocalFileSystemLoader(cfg.baseDir)
		if err != nil {
			return nil, fmt.Errorf("gotemplate: create local loader: %w", err)
		}
		loaders = append(loaders, loader)
	}
	if cfg.templates != nil {
		loaders = append(loaders, pongo2.NewFSLoader(cfg.templates))
	}

	engine := &Engine{
		templateSet: pongo2.NewSet("formgen", loaders...),
		templates:   make(map[string]*pongo2.Template),
		tplExt:      cfg.extension,
	}
	registerDefaultFilters()

	if err := engine.GlobalContext(cfg.globalData); err != nil {
		return nil, fmt.Errorf("gotemplate: apply global data: %w", err)
	}
	if len(cfg.templateFn) > 0 {
		for name, fn := range cfg.templateFn {
			if err := engine.registerTemplateFunc(name, fn); err != nil {
				return nil, fmt.Errorf("gotemplate: register template func %q: %w", name, err)
			}
		}
	}

	return engine, nil
}

// Render delegates to the wrapped engine.
func (e *Engine) Render(name string, data any, out ...io.Writer) (string, error) {
	if isTemplateContent(name) {
		return e.RenderString(name, data, out...)
	}
	return e.RenderTemplate(name, data, out...)
}

// RenderTemplate delegates to the wrapped engine.
func (e *Engine) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	if e == nil || e.templateSet == nil {
		return "", errors.New("gotemplate: engine is nil")
	}
	templatePath := name
	if !strings.HasSuffix(templatePath, e.tplExt) {
		templatePath += e.tplExt
	}

	tmpl, err := e.getTemplate(templatePath)
	if err != nil {
		return "", err
	}

	viewContext, err := convertToContext(data)
	if err != nil {
		return "", fmt.Errorf("gotemplate: convert data: %w", err)
	}

	var buf bytes.Buffer

	e.mu.RLock()
	err = tmpl.ExecuteWriter(viewContext, &buf)
	e.mu.RUnlock()

	if err != nil {
		return "", fmt.Errorf("gotemplate: execute template %q: %w", templatePath, err)
	}

	rendered := buf.String()
	if len(out) > 0 {
		for _, w := range out {
			if _, err := w.Write([]byte(rendered)); err != nil {
				return "", err
			}
		}
	}
	return rendered, nil
}

// RenderString delegates to the wrapped engine.
func (e *Engine) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	if e == nil || e.templateSet == nil {
		return "", errors.New("gotemplate: engine is nil")
	}

	tmpl, err := e.templateSet.FromString(templateContent)
	if err != nil {
		return "", fmt.Errorf("gotemplate: parse template string: %w", err)
	}

	viewContext, err := convertToContext(data)
	if err != nil {
		return "", fmt.Errorf("gotemplate: convert data: %w", err)
	}

	var buf bytes.Buffer

	e.mu.RLock()
	err = tmpl.ExecuteWriter(viewContext, &buf)
	e.mu.RUnlock()

	if err != nil {
		return "", fmt.Errorf("gotemplate: execute template string: %w", err)
	}

	rendered := buf.String()
	if len(out) > 0 {
		for _, w := range out {
			if _, err := w.Write([]byte(rendered)); err != nil {
				return "", err
			}
		}
	}
	return rendered, nil
}

// RegisterFilter registers template filters on the wrapped engine.
func (e *Engine) RegisterFilter(name string, fn func(input any, param any) (any, error)) error {
	if strings.TrimSpace(name) == "" || fn == nil {
		return errors.New("gotemplate: filter name and function required")
	}

	filter := func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		var paramVal any
		if param != nil {
			paramVal = param.Interface()
		}
		result, err := fn(in.Interface(), paramVal)
		if err != nil {
			return nil, &pongo2.Error{Sender: "custom_filter", OrigError: err}
		}
		return pongo2.AsValue(result), nil
	}

	if pongo2.FilterExists(name) {
		return fmt.Errorf("gotemplate: filter %q already exists", name)
	}
	return pongo2.RegisterFilter(name, filter)
}

// GlobalContext seeds global data on the wrapped engine.
func (e *Engine) GlobalContext(data any) error {
	if e == nil || e.templateSet == nil {
		return errors.New("gotemplate: engine is nil")
	}
	if data == nil {
		return nil
	}

	globalCtx, err := convertToContext(data)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.templateSet.Globals == nil {
		e.templateSet.Globals = make(pongo2.Context)
	}
	e.templateSet.Globals.Update(globalCtx)
	return nil
}

func (e *Engine) registerTemplateFunc(name string, fn any) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || fn == nil {
		return nil
	}

	if filter, ok := fn.(pongo2.FilterFunction); ok {
		if pongo2.FilterExists(trimmed) {
			return nil
		}
		return pongo2.RegisterFilter(trimmed, filter)
	}

	if !isCallable(fn) {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.templateSet.Globals == nil {
		e.templateSet.Globals = make(pongo2.Context)
	}
	e.templateSet.Globals[trimmed] = fn
	return nil
}

func (e *Engine) getTemplate(path string) (*pongo2.Template, error) {
	e.mu.RLock()
	if tmpl, ok := e.templates[path]; ok {
		e.mu.RUnlock()
		return tmpl, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	if tmpl, ok := e.templates[path]; ok {
		return tmpl, nil
	}

	tmpl, err := e.templateSet.FromFile(path)
	if err != nil {
		return nil, fmt.Errorf("gotemplate: load template %q: %w", path, err)
	}

	e.templates[path] = tmpl
	return tmpl, nil
}

func isTemplateContent(s string) bool {
	return strings.Contains(s, "{{") || strings.Contains(s, "{%")
}

func isCallable(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	return rv.IsValid() && rv.Kind() == reflect.Func
}

func convertToContext(data any) (pongo2.Context, error) {
	switch v := data.(type) {
	case nil:
		return pongo2.Context{}, nil
	case pongo2.Context:
		return convertMapToContext(map[string]any(v))
	case map[string]any:
		return convertMapToContext(v)
	default:
		m, err := jsonToMap(v)
		if err != nil {
			return nil, err
		}
		return convertMapToContext(m)
	}
}

func convertMapToContext(in map[string]any) (pongo2.Context, error) {
	out := make(pongo2.Context, len(in))
	for key, value := range in {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		converted, err := convertValue(value)
		if err != nil {
			return nil, err
		}
		out[key] = converted
	}
	return out, nil
}

func convertValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if isCallable(value) {
		return value, nil
	}

	switch v := value.(type) {
	case pongo2.Context:
		return convertMap(map[string]any(v))
	case map[string]any:
		return convertMap(v)
	case []any:
		return convertSlice(v)
	default:
		raw, err := jsonToAny(v)
		if err != nil {
			return nil, err
		}
		switch decoded := raw.(type) {
		case map[string]any:
			return convertMap(decoded)
		case []any:
			return convertSlice(decoded)
		default:
			return decoded, nil
		}
	}
}

func convertMap(in map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for key, value := range in {
		converted, err := convertValue(value)
		if err != nil {
			return nil, err
		}
		out[key] = converted
	}
	return out, nil
}

func convertSlice(in []any) ([]any, error) {
	out := make([]any, 0, len(in))
	for _, value := range in {
		converted, err := convertValue(value)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func jsonToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonToAny(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func registerDefaultFilters() {
	if !pongo2.FilterExists("trim") {
		_ = pongo2.RegisterFilter("trim", filterTrim)
	}
	if !pongo2.FilterExists("lowerfirst") {
		_ = pongo2.RegisterFilter("lowerfirst", filterLowerFirst)
	}
}

func filterTrim(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if in.Len() <= 0 {
		return pongo2.AsValue(""), nil
	}
	return pongo2.AsValue(strings.TrimSpace(in.String())), nil
}

func filterLowerFirst(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if in.Len() <= 0 {
		return pongo2.AsValue(""), nil
	}
	t := in.String()

	var (
		firstNonWhitespaceIndex int
		firstRune               rune
		firstRuneSize           int
	)

	for i, r := range t {
		if !strings.ContainsRune(" \t\n\r", r) {
			firstNonWhitespaceIndex = i
			firstRune = r
			firstRuneSize = utf8.RuneLen(r)
			break
		}
	}

	if firstRune == 0 {
		return pongo2.AsValue(t), nil
	}

	prefix := t[:firstNonWhitespaceIndex]
	loweredRune := strings.ToLower(string(firstRune))
	rest := t[firstNonWhitespaceIndex+firstRuneSize:]

	return pongo2.AsValue(prefix + loweredRune + rest), nil
}
