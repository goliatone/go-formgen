package jsonschema

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goliatone/go-formgen/pkg/schema"
)

type memoryLoader struct {
	docs  map[string]string
	calls map[string]int
}

func (m *memoryLoader) Load(ctx context.Context, src Source) (schema.Document, error) {
	if m.calls != nil {
		m.calls[src.Location()]++
	}
	raw, ok := m.docs[src.Location()]
	if !ok {
		return schema.Document{}, fmt.Errorf("missing document %q", src.Location())
	}
	return schema.NewDocument(src, []byte(raw))
}

type httpLoader struct {
	client *http.Client
}

func (h *httpLoader) Load(ctx context.Context, src Source) (schema.Document, error) {
	if src.Kind() != SourceKindURL {
		return schema.Document{}, errors.New("http loader: unsupported source kind")
	}
	if h.client == nil {
		return schema.Document{}, errors.New("http loader: client is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.Location(), nil)
	if err != nil {
		return schema.Document{}, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return schema.Document{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return schema.Document{}, fmt.Errorf("http loader: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return schema.Document{}, err
	}
	return schema.NewDocument(src, body)
}

func TestResolver_ResolveLocalRef(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {
    "name": {"type":"string"}
  },
  "type":"object",
  "properties": {
    "name": {"$ref": "#/$defs/name"}
  }
}`
	loader := &memoryLoader{docs: map[string]string{"root.json": root}}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	resolved, err := resolver.Resolve(context.Background(), doc, payload)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	props := resolved["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	if name["type"] != "string" {
		t.Fatalf("expected name type string, got %#v", name["type"])
	}
	if _, ok := name["$ref"]; ok {
		t.Fatalf("expected $ref to be resolved")
	}
}

func TestResolver_ResolveAnchorRef(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {
    "title": {"$anchor":"Title", "type":"string"}
  },
  "type":"object",
  "properties": {
    "title": {"$ref": "#Title"}
  }
}`
	loader := &memoryLoader{docs: map[string]string{"root.json": root}}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	resolved, err := resolver.Resolve(context.Background(), doc, payload)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	props := resolved["properties"].(map[string]any)
	title := props["title"].(map[string]any)
	if title["type"] != "string" {
		t.Fatalf("expected title type string, got %#v", title["type"])
	}
}

func TestResolver_CycleDetection(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {
    "a": {"$ref":"#/$defs/b"},
    "b": {"$ref":"#/$defs/a"}
  },
  "type":"object"
}`
	loader := &memoryLoader{docs: map[string]string{"root.json": root}}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), doc, payload)
	if err == nil {
		t.Fatalf("expected cycle error")
	}
}

func TestResolver_ResolveExternalFSRef(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "name": {"$ref": "defs.json#/$defs/name"}
  }
}`
	defs := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {
    "name": {"type":"string"}
  }
}`
	loader := &memoryLoader{docs: map[string]string{
		"root.json": root,
		"defs.json": defs,
	}}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	resolved, err := resolver.Resolve(context.Background(), doc, payload)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	props := resolved["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	if name["type"] != "string" {
		t.Fatalf("expected name type string, got %#v", name["type"])
	}
}

func TestResolver_PathTraversalGuard(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "secret": {"$ref": "../secret.json"}
  }
}`
	loader := &memoryLoader{docs: map[string]string{"schemas/root.json": root}}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("schemas/root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), doc, payload)
	if err == nil {
		t.Fatalf("expected path traversal error")
	}
}

func TestResolver_MaxDocuments(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "name": {"$ref": "defs.json#/$defs/name"}
  }
}`
	defs := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {"name": {"type":"string"}}
}`
	loader := &memoryLoader{docs: map[string]string{
		"root.json": root,
		"defs.json": defs,
	}}
	resolver := NewResolver(loader, ResolveOptions{MaxDocuments: 1})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), doc, payload)
	if err == nil {
		t.Fatalf("expected max documents error")
	}
}

func TestResolver_MaxDocumentBytes(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "name": {"$ref": "defs.json#/$defs/name"}
  }
}`
	defs := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {"name": {"type":"string", "description":"this-is-way-too-long"}}
}`
	loader := &memoryLoader{docs: map[string]string{
		"root.json": root,
		"defs.json": defs,
	}}
	resolver := NewResolver(loader, ResolveOptions{MaxDocumentBytes: 50})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), doc, payload)
	if err == nil {
		t.Fatalf("expected max document size error")
	}
}

func TestResolver_HTTPRefsDisabled(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "remote": {"$ref": "http://example.com/schema.json"}
  }
}`
	loader := &memoryLoader{docs: map[string]string{"root.json": root}}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), doc, payload)
	if err == nil {
		t.Fatalf("expected http refs disabled error")
	}
}

func TestResolver_HTTPRefsEnabled(t *testing.T) {
	remote := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"string"
}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(remote))
	}))
	defer server.Close()

	root := fmt.Sprintf(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "remote": {"$ref": "%s"}
  }
}`, server.URL)

	loader := &httpLoader{client: server.Client()}
	resolver := NewResolver(loader, ResolveOptions{AllowHTTPRefs: true})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	resolved, err := resolver.Resolve(context.Background(), doc, payload)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	props := resolved["properties"].(map[string]any)
	remoteProp := props["remote"].(map[string]any)
	if remoteProp["type"] != "string" {
		t.Fatalf("expected remote type string, got %#v", remoteProp["type"])
	}
}

func TestResolver_CachesDocuments(t *testing.T) {
	root := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "properties": {
    "first": {"$ref": "defs.json#/$defs/name"},
    "second": {"$ref": "defs.json#/$defs/name"}
  }
}`
	defs := `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$defs": {"name": {"type":"string"}}
}`
	loader := &memoryLoader{
		docs: map[string]string{
			"root.json": root,
			"defs.json": defs,
		},
		calls: make(map[string]int),
	}
	resolver := NewResolver(loader, ResolveOptions{})
	doc := MustNewDocument(SourceFromFS("root.json"), []byte(root))
	payload, err := parseJSONSchema(doc.Raw())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), doc, payload)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if loader.calls["defs.json"] != 1 {
		t.Fatalf("expected defs.json to load once, got %d", loader.calls["defs.json"])
	}
}
