package jsonschema

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/schema"
)

const (
	defaultMaxDocumentBytes = int64(5 << 20)
	defaultMaxDocuments     = 128
	defaultMaxRefDepth      = 64
)

// ResolveOptions configures JSON Schema ref resolution.
type ResolveOptions struct {
	// AllowHTTPRefs toggles HTTP/HTTPS ref resolution.
	AllowHTTPRefs bool
	// AllowPathTraversal permits refs to escape the root directory.
	AllowPathTraversal bool
	// MaxDocumentBytes caps the size of any single referenced document.
	MaxDocumentBytes int64
	// MaxDocuments caps the number of unique documents loaded during resolution.
	MaxDocuments int
	// MaxRefDepth caps the depth of $ref resolution chains.
	MaxRefDepth int
}

// Resolver resolves JSON Schema $ref references with guardrails.
type Resolver struct {
	loader Loader
	opts   ResolveOptions
}

type resolveSession struct {
	loader Loader
	opts   ResolveOptions
	cache  map[string]*resolvedDocument
	root   *resolvedRoot
}

type resolvedRoot struct {
	kind     schema.SourceKind
	location string
	baseDir  string
}

type resolvedDocument struct {
	key      string
	kind     schema.SourceKind
	location string
	baseDir  string
	data     map[string]any
	anchors  map[string]string
}

// NewResolver constructs a resolver with the supplied loader and options.
func NewResolver(loader Loader, opts ResolveOptions) *Resolver {
	if opts.MaxDocumentBytes <= 0 {
		opts.MaxDocumentBytes = defaultMaxDocumentBytes
	}
	if opts.MaxDocuments <= 0 {
		opts.MaxDocuments = defaultMaxDocuments
	}
	if opts.MaxRefDepth <= 0 {
		opts.MaxRefDepth = defaultMaxRefDepth
	}
	return &Resolver{loader: loader, opts: opts}
}

// Resolve expands $ref references for a parsed JSON Schema payload.
func (r *Resolver) Resolve(ctx context.Context, doc schema.Document, payload map[string]any) (map[string]any, error) {
	if r == nil {
		return nil, errors.New("jsonschema resolver: resolver is nil")
	}
	if r.loader == nil {
		return nil, errors.New("jsonschema resolver: loader is nil")
	}
	if doc.Source() == nil {
		return nil, errors.New("jsonschema resolver: source is nil")
	}

	session := &resolveSession{
		loader: r.loader,
		opts:   r.opts,
		cache:  make(map[string]*resolvedDocument),
	}

	root, err := session.prepareRoot(doc, payload)
	if err != nil {
		return nil, err
	}

	state := &resolveState{stack: make([]string, 0, 4), inStack: make(map[string]struct{})}
	resolved, err := session.resolveNode(ctx, root, root.data, state)
	if err != nil {
		return nil, err
	}

	output, ok := resolved.(map[string]any)
	if !ok {
		return nil, errors.New("jsonschema resolver: resolved root is not an object")
	}
	return output, nil
}

func (s *resolveSession) prepareRoot(doc schema.Document, payload map[string]any) (*resolvedDocument, error) {
	if payload == nil {
		return nil, errors.New("jsonschema resolver: payload is nil")
	}
	rootSrc := doc.Source()
	rootKey, rootLocation, baseDir, err := s.canonicalLocation(rootSrc)
	if err != nil {
		return nil, err
	}
	if int64(len(doc.Raw())) > s.opts.MaxDocumentBytes {
		return nil, fmt.Errorf("jsonschema resolver: document too large (%d bytes)", len(doc.Raw()))
	}

	s.root = &resolvedRoot{kind: rootSrc.Kind(), location: rootLocation, baseDir: baseDir}

	anchors := make(map[string]string)
	if err := indexAnchors(payload, "#", anchors); err != nil {
		return nil, err
	}

	rootDoc := &resolvedDocument{
		key:      rootKey,
		kind:     rootSrc.Kind(),
		location: rootLocation,
		baseDir:  baseDir,
		data:     payload,
		anchors:  anchors,
	}
	s.cache[rootKey] = rootDoc
	return rootDoc, nil
}

func (s *resolveSession) resolveNode(ctx context.Context, doc *resolvedDocument, node any, state *resolveState) (any, error) {
	switch typed := node.(type) {
	case map[string]any:
		if ref := strings.TrimSpace(readString(typed, "$ref")); ref != "" {
			refKey, refDoc, target, err := s.resolveRefTarget(ctx, doc, ref)
			if err != nil {
				return nil, err
			}
			if len(state.stack) >= s.opts.MaxRefDepth {
				return nil, fmt.Errorf("jsonschema resolver: ref depth exceeds %d", s.opts.MaxRefDepth)
			}
			if state.contains(refKey) {
				return nil, fmt.Errorf("jsonschema resolver: ref cycle detected at %s", ref)
			}
			merged, err := mergeRefTarget(target, typed)
			if err != nil {
				return nil, err
			}
			state.push(refKey)
			resolved, err := s.resolveNode(ctx, refDoc, merged, state)
			state.pop(refKey)
			if err != nil {
				return nil, err
			}
			return resolved, nil
		}

		resolved := make(map[string]any, len(typed))
		for key, value := range typed {
			switch key {
			case "$defs", "properties":
				items, ok := value.(map[string]any)
				if !ok {
					resolved[key] = value
					continue
				}
				child := make(map[string]any, len(items))
				for childKey, childValue := range items {
					resolvedChild, err := s.resolveNode(ctx, doc, childValue, state)
					if err != nil {
						return nil, err
					}
					child[childKey] = resolvedChild
				}
				resolved[key] = child
			case "items":
				resolvedChild, err := s.resolveNode(ctx, doc, value, state)
				if err != nil {
					return nil, err
				}
				resolved[key] = resolvedChild
			case "oneOf", "anyOf", "allOf":
				list, ok := value.([]any)
				if !ok {
					resolved[key] = value
					continue
				}
				out := make([]any, 0, len(list))
				for _, entry := range list {
					resolvedChild, err := s.resolveNode(ctx, doc, entry, state)
					if err != nil {
						return nil, err
					}
					out = append(out, resolvedChild)
				}
				resolved[key] = out
			default:
				resolved[key] = value
			}
		}
		return resolved, nil
	case []any:
		out := make([]any, 0, len(typed))
		for _, entry := range typed {
			resolvedChild, err := s.resolveNode(ctx, doc, entry, state)
			if err != nil {
				return nil, err
			}
			out = append(out, resolvedChild)
		}
		return out, nil
	default:
		return node, nil
	}
}

func (s *resolveSession) resolveRefTarget(ctx context.Context, doc *resolvedDocument, ref string) (string, *resolvedDocument, any, error) {
	refPath, fragment := splitRef(ref)
	if refPath == "" {
		refKey := doc.key + "#" + fragment
		resolved, err := s.resolveFragment(doc, fragment)
		return refKey, doc, resolved, err
	}

	parsed, err := url.Parse(refPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("jsonschema resolver: invalid ref %q", ref)
	}

	var target *resolvedDocument
	switch {
	case parsed.Scheme == "http" || parsed.Scheme == "https":
		if !s.opts.AllowHTTPRefs {
			return "", nil, nil, fmt.Errorf("jsonschema resolver: http refs disabled (%s)", ref)
		}
		src := SourceFromURL(parsed.String())
		target, err = s.loadDocument(ctx, src)
	case parsed.Scheme == "file":
		filePath := parsed.Path
		src := SourceFromFile(filePath)
		target, err = s.loadDocument(ctx, src)
	case parsed.Scheme != "":
		return "", nil, nil, fmt.Errorf("jsonschema resolver: unsupported ref scheme %q", parsed.Scheme)
	default:
		src, err := s.resolveRelativeSource(doc, parsed.Path)
		if err != nil {
			return "", nil, nil, err
		}
		target, err = s.loadDocument(ctx, src)
		if err != nil {
			return "", nil, nil, err
		}
	}
	if err != nil {
		return "", nil, nil, err
	}
	refKey := target.key + "#" + fragment
	resolved, err := s.resolveFragment(target, fragment)
	return refKey, target, resolved, err
}

func (s *resolveSession) resolveFragment(doc *resolvedDocument, fragment string) (any, error) {
	fragment = strings.TrimPrefix(fragment, "#")
	if fragment == "" {
		return cloneAny(doc.data), nil
	}
	if strings.HasPrefix(fragment, "/") {
		return resolveJSONPointer(doc.data, fragment)
	}
	pointer, ok := doc.anchors[fragment]
	if !ok {
		return nil, fmt.Errorf("jsonschema resolver: anchor %q not found", fragment)
	}
	if strings.HasPrefix(pointer, "#") {
		pointer = strings.TrimPrefix(pointer, "#")
	}
	if pointer == "" {
		return cloneAny(doc.data), nil
	}
	return resolveJSONPointer(doc.data, pointer)
}

func (s *resolveSession) loadDocument(ctx context.Context, src Source) (*resolvedDocument, error) {
	key, location, baseDir, err := s.canonicalLocation(src)
	if err != nil {
		return nil, err
	}

	if cached, ok := s.cache[key]; ok {
		return cached, nil
	}
	if len(s.cache) >= s.opts.MaxDocuments {
		return nil, fmt.Errorf("jsonschema resolver: exceeded max documents (%d)", s.opts.MaxDocuments)
	}

	doc, err := s.loader.Load(ctx, src)
	if err != nil {
		return nil, err
	}
	if int64(len(doc.Raw())) > s.opts.MaxDocumentBytes {
		return nil, fmt.Errorf("jsonschema resolver: document too large (%d bytes)", len(doc.Raw()))
	}
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		return nil, err
	}
	if err := validateDialect(payload); err != nil {
		return nil, err
	}
	anchors := make(map[string]string)
	if err := indexAnchors(payload, "#", anchors); err != nil {
		return nil, err
	}

	resolved := &resolvedDocument{
		key:      key,
		kind:     src.Kind(),
		location: location,
		baseDir:  baseDir,
		data:     payload,
		anchors:  anchors,
	}

	s.cache[key] = resolved

	return resolved, nil
}

func (s *resolveSession) resolveRelativeSource(doc *resolvedDocument, refPath string) (Source, error) {
	switch doc.kind {
	case SourceKindFile:
		resolved, err := s.cleanFilePath(doc.baseDir, refPath)
		if err != nil {
			return nil, err
		}
		return SourceFromFile(resolved), nil
	case SourceKindFS:
		resolved, err := s.cleanFSPath(doc.baseDir, refPath)
		if err != nil {
			return nil, err
		}
		return SourceFromFS(resolved), nil
	case SourceKindURL:
		if !s.opts.AllowHTTPRefs {
			return nil, fmt.Errorf("jsonschema resolver: http refs disabled (%s)", refPath)
		}
		base, err := url.Parse(doc.location)
		if err != nil {
			return nil, err
		}
		rel, err := url.Parse(refPath)
		if err != nil {
			return nil, err
		}
		return SourceFromURL(base.ResolveReference(rel).String()), nil
	default:
		return nil, errors.New("jsonschema resolver: unsupported source kind")
	}
}

func (s *resolveSession) canonicalLocation(src Source) (string, string, string, error) {
	if src == nil {
		return "", "", "", errors.New("jsonschema resolver: source is nil")
	}
	location := src.Location()
	switch src.Kind() {
	case SourceKindFile:
		abs, err := filepath.Abs(location)
		if err != nil {
			return "", "", "", err
		}
		base := filepath.Dir(abs)
		return "file:" + abs, abs, base, nil
	case SourceKindFS:
		cleaned := path.Clean(strings.TrimPrefix(location, "/"))
		base := path.Dir(cleaned)
		return "fs:" + cleaned, cleaned, base, nil
	case SourceKindURL:
		return "url:" + location, location, path.Dir(location), nil
	default:
		return "", "", "", errors.New("jsonschema resolver: unsupported source kind")
	}
}

func (s *resolveSession) cleanFilePath(baseDir, refPath string) (string, error) {
	candidate := refPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(baseDir, refPath)
	}
	candidate = filepath.Clean(candidate)
	if s.opts.AllowPathTraversal {
		return candidate, nil
	}
	root := baseDir
	if s.root != nil {
		root = s.root.baseDir
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("jsonschema resolver: ref path escapes root (%s)", refPath)
	}
	return candidate, nil
}

func (s *resolveSession) cleanFSPath(baseDir, refPath string) (string, error) {
	candidate := path.Clean(path.Join(baseDir, refPath))
	candidate = strings.TrimPrefix(candidate, "/")
	if s.opts.AllowPathTraversal {
		return candidate, nil
	}
	root := baseDir
	if s.root != nil {
		root = s.root.baseDir
	}
	root = strings.TrimPrefix(path.Clean(root), "/")
	if root == "." {
		root = ""
	}
	if root == "" {
		if strings.HasPrefix(candidate, "..") {
			return "", fmt.Errorf("jsonschema resolver: ref path escapes root (%s)", refPath)
		}
		return candidate, nil
	}
	if candidate == root || strings.HasPrefix(candidate, root+"/") {
		return candidate, nil
	}
	return "", fmt.Errorf("jsonschema resolver: ref path escapes root (%s)", refPath)
}

func splitRef(ref string) (string, string) {
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func resolveJSONPointer(root any, pointer string) (any, error) {
	if pointer == "" || pointer == "#" {
		return cloneAny(root), nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("jsonschema resolver: invalid json pointer %q", pointer)
	}

	current := root
	parts := strings.Split(pointer, "/")[1:]
	for _, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, err
		}
		decoded = strings.ReplaceAll(decoded, "~1", "/")
		decoded = strings.ReplaceAll(decoded, "~0", "~")

		switch typed := current.(type) {
		case map[string]any:
			value, ok := typed[decoded]
			if !ok {
				return nil, fmt.Errorf("jsonschema resolver: pointer %q not found", pointer)
			}
			current = value
		case []any:
			idx, err := strconv.Atoi(decoded)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, fmt.Errorf("jsonschema resolver: pointer %q out of range", pointer)
			}
			current = typed[idx]
		default:
			return nil, fmt.Errorf("jsonschema resolver: pointer %q invalid", pointer)
		}
	}

	return cloneAny(current), nil
}

func indexAnchors(node any, pointer string, anchors map[string]string) error {
	switch typed := node.(type) {
	case map[string]any:
		if raw, ok := typed["$anchor"]; ok {
			name, ok := raw.(string)
			name = strings.TrimSpace(name)
			if ok && name != "" {
				if _, exists := anchors[name]; exists {
					return fmt.Errorf("jsonschema resolver: duplicate anchor %q", name)
				}
				anchors[name] = pointer
			}
		}
		for key, value := range typed {
			if isVendorExtension(key) {
				continue
			}
			childPointer := joinPath(pointer, key)
			if err := indexAnchors(value, childPointer, anchors); err != nil {
				return err
			}
		}
	case []any:
		for idx, value := range typed {
			childPointer := joinPath(pointer, strconv.Itoa(idx))
			if err := indexAnchors(value, childPointer, anchors); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeRefTarget(target any, refObj map[string]any) (any, error) {
	merged := cloneAny(target)
	if mergedMap, ok := merged.(map[string]any); ok {
		for key, value := range refObj {
			if key == "$ref" {
				continue
			}
			if !isAllowedRefSibling(key) {
				return nil, fmt.Errorf("jsonschema resolver: unsupported $ref sibling %q", key)
			}
			mergedMap[key] = value
		}
		return mergedMap, nil
	}
	for key := range refObj {
		if key != "$ref" {
			return nil, fmt.Errorf("jsonschema resolver: $ref target is not an object")
		}
	}
	return merged, nil
}

func isAllowedRefSibling(key string) bool {
	if key == "title" || key == "description" || key == "default" {
		return true
	}
	return isVendorExtension(key)
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			out[key] = cloneAny(val)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for idx, val := range typed {
			out[idx] = cloneAny(val)
		}
		return out
	default:
		return typed
	}
}

type resolveState struct {
	stack   []string
	inStack map[string]struct{}
}

func (s *resolveState) push(ref string) {
	s.stack = append(s.stack, ref)
	if s.inStack == nil {
		s.inStack = make(map[string]struct{})
	}
	s.inStack[ref] = struct{}{}
}

func (s *resolveState) pop(ref string) {
	if len(s.stack) == 0 {
		return
	}
	last := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	delete(s.inStack, last)
	if ref != last {
		delete(s.inStack, ref)
	}
}

func (s *resolveState) contains(ref string) bool {
	_, ok := s.inStack[ref]
	return ok
}
