package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	defaultSource := exampleutil.FixturePath("petstore.json")

	var (
		addrFlag      = flag.String("addr", ":8080", "HTTP listen address")
		sourceFlag    = flag.String("source", defaultSource, "Default OpenAPI source (file path or URL)")
		rendererFlag  = flag.String("renderer", "vanilla", "Default renderer name")
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

	server := &formServer{
		generator: formgen.NewOrchestrator(
			orchestrator.WithLoader(loader),
			orchestrator.WithParser(formgen.NewParser()),
			orchestrator.WithModelBuilder(model.NewBuilder()),
			orchestrator.WithRegistry(registry),
			orchestrator.WithDefaultRenderer(*rendererFlag),
		),
		loader:          loader,
		registry:        registry,
		cache:           newDocumentCache(),
		defaultSource:   *sourceFlag,
		defaultRenderer: *rendererFlag,
	}

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(preact.AssetsFS()))))
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
	generator       *orchestrator.Orchestrator
	loader          pkgopenapi.Loader
	registry        *render.Registry
	cache           *documentCache
	defaultSource   string
	defaultRenderer string
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
			operation = "createPet"
		}

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

		w.Header().Set("Content-Type", renderer.ContentType())
		if _, err := w.Write(output); err != nil {
			log.Printf("write response: %v", err)
		}
	})
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
