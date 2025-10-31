package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/goliatone/formgen"
	"github.com/goliatone/formgen/examples/internal/exampleutil"
	"github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/preact"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
)

const vanillaRuntimeBootstrap = `
<script src="/runtime/formgen-relationships.min.js" defer></script>
<script>
window.addEventListener('DOMContentLoaded', function () {
  if (!window.FormgenRelationships || typeof window.FormgenRelationships.initRelationships !== 'function') {
    console.warn('formgen runtime bundle not loaded; run "npm run build" in the repo root to generate dist/browser assets');
    return;
  }
  window.FormgenRelationships.initRelationships().catch(function (error) {
    console.error('initRelationships failed', error);
  });
});
</script>
`

func defaultSchemaPath() string {
	fixture := exampleutil.FixturePath("petstore.json")
	candidate := filepath.Clean(filepath.Join(filepath.Dir(fixture), "..", "http", "schema.json"))
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return fixture
}

func main() {
	defaultSource := defaultSchemaPath()

	var (
		addrFlag      = flag.String("addr", ":8080", "HTTP listen address")
		sourceFlag    = flag.String("source", defaultSource, "Default OpenAPI source (file path or URL)")
		rendererFlag  = flag.String("renderer", "vanilla", "Default renderer name")
		operationFlag = flag.String("operation", "createArticle", "Default operation ID")
		shutdownGrace = flag.Duration("grace", 5*time.Second, "Shutdown grace period")
	)
	flag.Parse()

	registry := render.NewRegistry()
	registry.MustRegister(mustVanilla())
	registry.MustRegister(mustPreact())

	if !registry.Has(*rendererFlag) {
		log.Fatalf("default renderer %q is not registered", *rendererFlag)
	}

	loader := formgen.NewLoader(
		pkgopenapi.WithDefaultSources(),
		pkgopenapi.WithHTTPClient(http.DefaultClient),
	)
	parser := formgen.NewParser()
	builder := model.NewBuilder()

	defaultOperation := strings.TrimSpace(*operationFlag)
	if defaultOperation == "" {
		defaultOperation = "createArticle"
	}

	server := &formServer{
		generator: formgen.NewOrchestrator(
			orchestrator.WithLoader(loader),
			orchestrator.WithParser(parser),
			orchestrator.WithModelBuilder(builder),
			orchestrator.WithRegistry(registry),
			orchestrator.WithDefaultRenderer(*rendererFlag),
		),
		loader:           loader,
		parser:           parser,
		builder:          builder,
		registry:         registry,
		cache:            newDocumentCache(),
		defaultSource:    *sourceFlag,
		defaultRenderer:  *rendererFlag,
		defaultOperation: defaultOperation,
	}

	relationships := newRelationshipDataset()

	mux := http.NewServeMux()
	relationships.register(mux)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(preact.AssetsFS()))))
	if _, err := os.Stat("client/dist/browser"); err == nil {
		mux.Handle("/runtime/", http.StripPrefix("/runtime/", http.FileServer(http.Dir("client/dist/browser"))))
	} else {
		log.Printf("relationship runtime bundle not found on disk; run `cd client && npm run build` to expose /runtime assets")
	}
	mux.Handle("/form", server.formHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	httpServer := &http.Server{
		Addr:    *addrFlag,
		Handler: mux,
	}

	log.Printf("listening on %s (default source %s renderer %s)", *addrFlag, *sourceFlag, *rendererFlag)

	errChan := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errChan:
		log.Fatalf("listen: %v", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), *shutdownGrace)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

type formServer struct {
	generator        *orchestrator.Orchestrator
	loader           pkgopenapi.Loader
	parser           pkgopenapi.Parser
	builder          model.Builder
	registry         *render.Registry
	cache            *documentCache
	defaultSource    string
	defaultRenderer  string
	defaultOperation string
}

func (s *formServer) formHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		sourceRaw := query.Get("source")
		if strings.TrimSpace(sourceRaw) == "" {
			sourceRaw = s.defaultSource
		}
		rendererName := query.Get("renderer")
		if strings.TrimSpace(rendererName) == "" {
			rendererName = s.defaultRenderer
		}
		operation := query.Get("operation")
		if strings.TrimSpace(operation) == "" {
			operation = s.defaultOperation
		}
		format := strings.ToLower(strings.TrimSpace(query.Get("format")))

		source, cacheKey, err := exampleutil.ResolveSource(sourceRaw)
		if err != nil {
			http.Error(w, fmt.Sprintf("resolve source: %v", err), http.StatusBadRequest)
			return
		}

		var document pkgopenapi.Document
		if cached, ok := s.cache.Get(cacheKey); ok {
			document = cached
		} else {
			document, err = s.loader.Load(r.Context(), source)
			if err != nil {
				http.Error(w, fmt.Sprintf("load document: %v", err), http.StatusBadGateway)
				return
			}
			s.cache.Set(cacheKey, document)
		}

		if format == "json" {
			form, err := s.buildFormModel(r.Context(), document, operation)
			if err != nil {
				http.Error(w, fmt.Sprintf("build form model: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(form); err != nil {
				log.Printf("write json response: %v", err)
			}
			return
		}

		renderer, err := s.registry.Get(rendererName)
		if err != nil {
			http.Error(w, fmt.Sprintf("renderer %q not found", rendererName), http.StatusNotFound)
			return
		}

		output, err := s.generator.Generate(r.Context(), orchestrator.Request{
			Document:    &document,
			OperationID: operation,
			Renderer:    rendererName,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("generate: %v", err), http.StatusInternalServerError)
			return
		}

		if renderer.Name() == "vanilla" {
			output = append(output, []byte(vanillaRuntimeBootstrap)...)
		}

		w.Header().Set("Content-Type", renderer.ContentType())
		if _, err := w.Write(output); err != nil {
			log.Printf("write response: %v", err)
		}
	})
}

func (s *formServer) buildFormModel(ctx context.Context, document pkgopenapi.Document, operation string) (model.FormModel, error) {
	if s.parser == nil || s.builder == nil {
		return model.FormModel{}, fmt.Errorf("form server missing parser or builder")
	}
	operations, err := s.parser.Operations(ctx, document)
	if err != nil {
		return model.FormModel{}, fmt.Errorf("parse operations: %w", err)
	}
	op, ok := operations[operation]
	if !ok {
		return model.FormModel{}, fmt.Errorf("operation %q not found", operation)
	}
	form, err := s.builder.Build(op)
	if err != nil {
		return model.FormModel{}, fmt.Errorf("build form model: %w", err)
	}
	return form, nil
}

type relationshipDataset struct {
	authors    []authorRecord
	categories []categoryRecord
	managers   []managerRecord
	tags       []tagRecord
}

func newRelationshipDataset() *relationshipDataset {
	return &relationshipDataset{
		authors: []authorRecord{
			{
				ID:       "11111111-1111-4111-8111-111111111111",
				FullName: "Ada Lovelace",
				Email:    "ada@example.com",
				TenantID: "northwind",
				Profile: authorProfile{
					Bio:     "First programmer and resident polymath.",
					Twitter: "@ada",
				},
			},
			{
				ID:       "22222222-2222-4222-8222-222222222222",
				FullName: "Claude Shannon",
				Email:    "claude@example.com",
				TenantID: "northwind",
				Profile: authorProfile{
					Bio:     "Father of information theory.",
					Twitter: "@entropy",
				},
			},
			{
				ID:       "33333333-3333-4333-8333-333333333333",
				FullName: "Octavia E. Butler",
				Email:    "octavia@example.com",
				TenantID: "lumen",
				Profile: authorProfile{
					Bio:     "Visionary science fiction author.",
					Twitter: "@patternist",
				},
			},
			{
				ID:       "44444444-4444-4444-8444-444444444444",
				FullName: "Ida B. Wells",
				Email:    "ida@example.com",
				TenantID: "lumen",
				Profile: authorProfile{
					Bio:     "Investigative journalist and civil rights pioneer.",
					Twitter: "@idabwells",
				},
			},
		},
		categories: []categoryRecord{
			{ID: "55555555-5555-4555-8555-555555555555", Name: "Engineering"},
			{ID: "66666666-6666-4666-8666-666666666666", Name: "Culture"},
			{ID: "77777777-7777-4777-8777-777777777777", Name: "Operations"},
		},
		managers: []managerRecord{
			{ID: "88888888-8888-4888-8888-888888888888", Name: "Grace Hopper"},
			{ID: "99999999-9999-4999-8999-999999999999", Name: "Radia Perlman"},
		},
		tags: []tagRecord{
			{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Label: "feature", Description: "Feature launch related content"},
			{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Label: "announcement", Description: "Company announcements"},
			{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Label: "editorial", Description: "Long-form editorial work"},
			{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", Label: "people", Description: "People and culture stories"},
		},
	}
}

func (d *relationshipDataset) register(mux *http.ServeMux) {
	if mux == nil {
		return
	}
	mux.HandleFunc("/api/authors", d.handleAuthors)
	mux.HandleFunc("/api/categories", d.handleCategories)
	mux.HandleFunc("/api/managers", d.handleManagers)
	mux.HandleFunc("/api/tags", d.handleTags)
}

func (d *relationshipDataset) handleAuthors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	tenantFilter := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	results := make([]authorRecord, 0, len(d.authors))
	for _, author := range d.authors {
		if tenantFilter != "" && !strings.EqualFold(author.TenantID, tenantFilter) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(author.FullName), search) {
			continue
		}
		results = append(results, author)
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	if search == "" {
		writeRelationshipData(w, d.categories)
		return
	}

	results := make([]categoryRecord, 0, len(d.categories))
	for _, category := range d.categories {
		if strings.Contains(strings.ToLower(category.Name), search) {
			results = append(results, category)
		}
	}
	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleManagers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	writeRelationshipData(w, d.managers)
}

func (d *relationshipDataset) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	results := make([]tagRecord, 0, len(d.tags))
	for _, tag := range d.tags {
		if search != "" && !strings.Contains(strings.ToLower(tag.Label), search) {
			continue
		}
		results = append(results, tag)
	}
	writeRelationshipData(w, results)
}

func writeRelationshipData(w http.ResponseWriter, items any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"data": items}); err != nil {
		log.Printf("write relationship data: %v", err)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

type authorRecord struct {
	ID       string        `json:"id"`
	FullName string        `json:"full_name"`
	Email    string        `json:"email"`
	TenantID string        `json:"tenant_id"`
	Profile  authorProfile `json:"profile"`
}

type authorProfile struct {
	Bio     string `json:"bio"`
	Twitter string `json:"twitter"`
}

type categoryRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type managerRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tagRecord struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type documentCache struct {
	mu    sync.RWMutex
	items map[string]pkgopenapi.Document
}

func newDocumentCache() *documentCache {
	return &documentCache{
		items: make(map[string]pkgopenapi.Document),
	}
}

func (c *documentCache) Get(key string) (pkgopenapi.Document, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	doc, ok := c.items[key]
	return doc, ok
}

func (c *documentCache) Set(key string, doc pkgopenapi.Document) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = doc
}

func mustVanilla() render.Renderer {
	r, err := vanilla.New(vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()))
	if err != nil {
		log.Fatalf("vanilla renderer: %v", err)
	}
	return r
}

func mustPreact() render.Renderer {
	r, err := preact.New(preact.WithAssetURLPrefix("/assets"))
	if err != nil {
		log.Fatalf("preact renderer: %v", err)
	}
	return r
}
