package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
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
	"github.com/goliatone/formgen/pkg/renderers/vanilla/components"
)

const vanillaRuntimeBootstrap = `
<script src="/runtime/formgen-relationships.min.js" defer></script>
<script src="/runtime/formgen-behaviors.min.js" defer></script>
<script>
window.addEventListener('DOMContentLoaded', function () {
  if (!window.FormgenRelationships || typeof window.FormgenRelationships.initRelationships !== 'function') {
    console.warn('formgen runtime bundle not loaded; run "npm run build" in the repo root to generate dist/browser assets');
    return;
  }
  if (typeof window.FormgenRelationships.registerComponent === 'function') {
    window.FormgenRelationships.registerComponent('status-pill', function (context) {
      var element = context.element;
      var pills = Array.from(element.querySelectorAll('[data-pill]'));
      var radios = Array.from(element.querySelectorAll('input[type="radio"]'));
      var activate = function () {
        var selected = element.querySelector('input[type="radio"]:checked');
        pills.forEach(function (pill) {
          var input = pill.querySelector('input[type="radio"]');
          var active = input === selected;
          pill.classList.toggle('bg-blue-600', active);
          pill.classList.toggle('text-white', active);
          pill.classList.toggle('border-blue-600', active);
          pill.classList.toggle('bg-gray-100', !active);
          pill.classList.toggle('text-gray-700', !active);
        });
      };
      radios.forEach(function (input) {
        input.addEventListener('change', activate);
      });
      activate();
    });
  }
  window.FormgenRelationships.initRelationships().catch(function (error) {
    console.error('initRelationships failed', error);
  });
  if (window.FormgenBehaviors && typeof window.FormgenBehaviors.initBehaviors === 'function') {
    window.FormgenBehaviors.initBehaviors();
  } else {
    console.warn('formgen behaviors bundle not loaded; run "npm run build" to generate dist/browser assets');
  }
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

func defaultUISchemaDir(schemaPath string) string {
	path := strings.TrimSpace(schemaPath)
	if path == "" {
		return ""
	}
	candidate := filepath.Clean(filepath.Join(filepath.Dir(path), "ui"))
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return ""
	}
	return candidate
}

func main() {
	defaultSource := defaultSchemaPath()

	var (
		addrFlag      = flag.String("addr", ":8080", "HTTP listen address")
		sourceFlag    = flag.String("source", defaultSource, "Default OpenAPI source (file path or URL)")
		rendererFlag  = flag.String("renderer", "vanilla", "Default renderer name")
		operationFlag = flag.String("operation", "post-book:create", "Default operation ID")
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
	uiSchemaDir := defaultUISchemaDir(*sourceFlag)
	options := []orchestrator.Option{
		orchestrator.WithLoader(loader),
		orchestrator.WithParser(parser),
		orchestrator.WithModelBuilder(builder),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(*rendererFlag),
	}
	if uiSchemaDir != "" {
		options = append(options, orchestrator.WithUISchemaFS(os.DirFS(uiSchemaDir)))
	}

	defaultOperation := strings.TrimSpace(*operationFlag)
	if defaultOperation == "" {
		defaultOperation = "post-book:create"
	}

	server := &formServer{
		generator:        formgen.NewOrchestrator(options...),
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
	mux.Handle("/runtime/", http.StripPrefix("/runtime/", http.FileServer(http.FS(vanilla.AssetsFS()))))
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
	authors        []authorRecord
	categories     []categoryRecord
	managers       []managerRecord
	tags           []tagRecord
	publishers     []publishingHouseRecord
	authorProfiles []authorProfileRecord
	books          []bookRecord
	chapters       []chapterRecord
	headquarters   []headquartersRecord
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
		publishers: []publishingHouseRecord{
			{
				ID:              "aaaa1111-aaaa-4111-8111-aaaaaaaaaaaa",
				Name:            "Atlas Press",
				ImprintPrefix:   "AT",
				EstablishedYear: 1978,
				Headquarters: headquartersRecord{
					ID:          "hq-001",
					AddressLine: "101 Memory Ln",
					City:        "New York",
					Country:     "USA",
					OpenedAt:    "1978-03-14T08:00:00Z",
					PublisherID: "aaaa1111-aaaa-4111-8111-aaaaaaaaaaaa",
				},
			},
			{
				ID:              "bbbb2222-bbbb-4222-8222-bbbbbbbbbbbb",
				Name:            "Lumen House",
				ImprintPrefix:   "LH",
				EstablishedYear: 1994,
				Headquarters: headquartersRecord{
					ID:          "hq-002",
					AddressLine: "88 Horizon Ave",
					City:        "San Francisco",
					Country:     "USA",
					OpenedAt:    "1995-01-09T09:30:00Z",
					PublisherID: "bbbb2222-bbbb-4222-8222-bbbbbbbbbbbb",
				},
			},
			{
				ID:              "cccc3333-cccc-4333-8333-cccccccccccc",
				Name:            "Northwind Publishing",
				ImprintPrefix:   "NW",
				EstablishedYear: 1965,
				Headquarters: headquartersRecord{
					ID:          "hq-003",
					AddressLine: "42 Cyclone Rd",
					City:        "Chicago",
					Country:     "USA",
					OpenedAt:    "1966-07-22T10:15:00Z",
					PublisherID: "cccc3333-cccc-4333-8333-cccccccccccc",
				},
			},
		},
		authorProfiles: []authorProfileRecord{
			{
				ID:            "profile-ada",
				AuthorID:      "11111111-1111-4111-8111-111111111111",
				Biography:     "Ada merges analytical engines with poetic imagination.",
				FavoriteGenre: "Mathematical fiction",
				WritingStyle:  "Lyric technical essays",
			},
			{
				ID:            "profile-claude",
				AuthorID:      "22222222-2222-4222-8222-222222222222",
				Biography:     "Claude documents the language of information itself.",
				FavoriteGenre: "Technical deep dives",
				WritingStyle:  "Succinct and precise",
			},
			{
				ID:            "profile-octavia",
				AuthorID:      "33333333-3333-4333-8333-333333333333",
				Biography:     "Octavia chronicles speculative futures rooted in realism.",
				FavoriteGenre: "Speculative fiction",
				WritingStyle:  "World-building epics",
			},
		},
		books: []bookRecord{
			{
				ID:          "book-roses",
				Title:       "Roses of Entropy",
				ISBN:        "978-0-00-000001-0",
				AuthorID:    "22222222-2222-4222-8222-222222222222",
				PublisherID: "aaaa1111-aaaa-4111-8111-aaaaaaaaaaaa",
				ReleaseDate: "2021-03-03",
				Status:      "published",
				Tags:        []string{"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"},
			},
			{
				ID:          "book-horizon",
				Title:       "The Horizon Tapes",
				ISBN:        "978-0-00-000002-7",
				AuthorID:    "33333333-3333-4333-8333-333333333333",
				PublisherID: "bbbb2222-bbbb-4222-8222-bbbbbbbbbbbb",
				ReleaseDate: "2022-10-12",
				Status:      "in_review",
				Tags:        []string{"cccccccc-cccc-4ccc-8ccc-cccccccccccc"},
			},
			{
				ID:          "book-light",
				Title:       "Light Cones and City Streets",
				ISBN:        "978-0-00-000003-4",
				AuthorID:    "11111111-1111-4111-8111-111111111111",
				PublisherID: "cccc3333-cccc-4333-8333-cccccccccccc",
				ReleaseDate: "2020-06-21",
				Status:      "published",
				Tags:        []string{"dddddddd-dddd-4ddd-8ddd-dddddddddddd"},
			},
		},
		chapters: []chapterRecord{
			{ID: "chapter-roses-1", BookID: "book-roses", Title: "Signals", WordCount: 3200},
			{ID: "chapter-roses-2", BookID: "book-roses", Title: "Noise", WordCount: 4100},
			{ID: "chapter-horizon-1", BookID: "book-horizon", Title: "Dawn", WordCount: 2800},
			{ID: "chapter-light-1", BookID: "book-light", Title: "City Center", WordCount: 3600},
		},
		headquarters: []headquartersRecord{
			{
				ID:          "hq-001",
				AddressLine: "101 Memory Ln",
				City:        "New York",
				Country:     "USA",
				OpenedAt:    "1978-03-14T08:00:00Z",
				PublisherID: "aaaa1111-aaaa-4111-8111-aaaaaaaaaaaa",
			},
			{
				ID:          "hq-002",
				AddressLine: "88 Horizon Ave",
				City:        "San Francisco",
				Country:     "USA",
				OpenedAt:    "1995-01-09T09:30:00Z",
				PublisherID: "bbbb2222-bbbb-4222-8222-bbbbbbbbbbbb",
			},
			{
				ID:          "hq-003",
				AddressLine: "42 Cyclone Rd",
				City:        "Chicago",
				Country:     "USA",
				OpenedAt:    "1966-07-22T10:15:00Z",
				PublisherID: "cccc3333-cccc-4333-8333-cccccccccccc",
			},
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
	mux.HandleFunc("/api/publishing-houses", d.handlePublishingHouses)
	mux.HandleFunc("/api/author-profiles", d.handleAuthorProfiles)
	mux.HandleFunc("/api/books", d.handleBooks)
	mux.HandleFunc("/api/chapters", d.handleChapters)
	mux.HandleFunc("/api/headquarters", d.handleHeadquarters)
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

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(a authorRecord) string { return a.ID }, func(a authorRecord) string {
			return a.FullName
		})
		return
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
		if wantsOptionsFormat(r) {
			writeRelationshipOptions(w, d.categories, func(c categoryRecord) string { return c.ID }, func(c categoryRecord) string {
				return c.Name
			})
			return
		}
		writeRelationshipData(w, d.categories)
		return
	}

	results := make([]categoryRecord, 0, len(d.categories))
	for _, category := range d.categories {
		if strings.Contains(strings.ToLower(category.Name), search) {
			results = append(results, category)
		}
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(c categoryRecord) string { return c.ID }, func(c categoryRecord) string {
			return c.Name
		})
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleManagers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, d.managers, func(m managerRecord) string { return m.ID }, func(m managerRecord) string {
			return m.Name
		})
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

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(t tagRecord) string { return t.ID }, func(t tagRecord) string {
			return t.Label
		})
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handlePublishingHouses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	results := make([]publishingHouseRecord, 0, len(d.publishers))
	for _, publisher := range d.publishers {
		if search == "" {
			results = append(results, publisher)
			continue
		}
		if strings.Contains(strings.ToLower(publisher.Name), search) ||
			strings.Contains(strings.ToLower(publisher.ImprintPrefix), search) {
			results = append(results, publisher)
		}
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(p publishingHouseRecord) string { return p.ID }, func(p publishingHouseRecord) string {
			return p.Name
		})
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleAuthorProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	authorID := strings.TrimSpace(r.URL.Query().Get("author_id"))

	results := make([]authorProfileRecord, 0, len(d.authorProfiles))
	for _, profile := range d.authorProfiles {
		if authorID != "" && !strings.EqualFold(profile.AuthorID, authorID) {
			continue
		}
		if search != "" {
			match := strings.Contains(strings.ToLower(profile.AuthorID), search) ||
				strings.Contains(strings.ToLower(profile.Biography), search) ||
				strings.Contains(strings.ToLower(profile.FavoriteGenre), search)
			if !match {
				continue
			}
		}
		results = append(results, profile)
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(p authorProfileRecord) string { return p.ID }, func(p authorProfileRecord) string {
			return p.AuthorID
		})
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleBooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	results := make([]bookRecord, 0, len(d.books))
	for _, book := range d.books {
		if search == "" || strings.Contains(strings.ToLower(book.Title), search) || strings.Contains(strings.ToLower(book.ISBN), search) {
			results = append(results, book)
		}
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(b bookRecord) string { return b.ID }, func(b bookRecord) string { return b.Title })
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleChapters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	bookID := strings.TrimSpace(r.URL.Query().Get("book_id"))

	results := make([]chapterRecord, 0, len(d.chapters))
	for _, chapter := range d.chapters {
		if bookID != "" && !strings.EqualFold(chapter.BookID, bookID) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(chapter.Title), search) {
			continue
		}
		results = append(results, chapter)
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(c chapterRecord) string { return c.ID }, func(c chapterRecord) string { return c.Title })
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleHeadquarters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	results := make([]headquartersRecord, 0, len(d.headquarters))
	for _, hq := range d.headquarters {
		if search == "" || strings.Contains(strings.ToLower(hq.AddressLine), search) || strings.Contains(strings.ToLower(hq.City), search) {
			results = append(results, hq)
		}
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(h headquartersRecord) string { return h.ID }, func(h headquartersRecord) string { return h.AddressLine })
		return
	}

	writeRelationshipData(w, results)
}

func wantsOptionsFormat(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "options")
}

func writeRelationshipOptions[T any](w http.ResponseWriter, items []T, value func(T) string, label func(T) string) {
	w.Header().Set("Content-Type", "application/json")

	payload := make([]map[string]string, 0, len(items))
	for _, item := range items {
		payload = append(payload, map[string]string{
			"value": value(item),
			"label": label(item),
		})
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write relationship options: %v", err)
	}
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

type publishingHouseRecord struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	ImprintPrefix   string             `json:"imprint_prefix,omitempty"`
	EstablishedYear int                `json:"established_year,omitempty"`
	Headquarters    headquartersRecord `json:"headquarters"`
}

type headquartersRecord struct {
	ID           string `json:"id"`
	AddressLine  string `json:"address_line1"`
	AddressLine2 string `json:"address_line2,omitempty"`
	City         string `json:"city"`
	Country      string `json:"country"`
	OpenedAt     string `json:"opened_at,omitempty"`
	PublisherID  string `json:"publisher_id,omitempty"`
}

type authorProfileRecord struct {
	ID            string `json:"id"`
	AuthorID      string `json:"author_id"`
	Biography     string `json:"biography"`
	FavoriteGenre string `json:"favorite_genre"`
	WritingStyle  string `json:"writing_style"`
}

type bookRecord struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	ISBN        string   `json:"isbn"`
	AuthorID    string   `json:"author_id"`
	PublisherID string   `json:"publisher_id"`
	ReleaseDate string   `json:"release_date"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags,omitempty"`
}

type chapterRecord struct {
	ID        string `json:"id"`
	BookID    string `json:"book_id"`
	Title     string `json:"title"`
	WordCount int    `json:"word_count"`
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
	registry := components.NewDefaultRegistry()
	registry.MustRegister("empty", components.Descriptor{
		Renderer: emptyComponentRenderer,
	})
	registry.MustRegister("status-pill", components.Descriptor{
		Renderer: statusPillRenderer,
	})

	r, err := vanilla.New(
		vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()),
		vanilla.WithDefaultStyles(),
		vanilla.WithComponentRegistry(registry),
	)
	if err != nil {
		log.Fatalf("vanilla renderer: %v", err)
	}
	return r
}

func emptyComponentRenderer(_ *bytes.Buffer, _ model.Field, _ components.ComponentData) error {
	return nil
}

type pillOption struct {
	Label string
	Value string
}

func statusPillRenderer(buf *bytes.Buffer, field model.Field, data components.ComponentData) error {
	var builder strings.Builder
	options := extractPillOptions(field, data.Config)
	current := toString(field.Default)

	if len(options) == 0 {
		builder.WriteString(`<input type="text" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500">`)
		buf.WriteString(builder.String())
		return nil
	}

	builder.WriteString(`<div class="flex flex-wrap gap-2" data-status-pill="true">`)
	for idx, option := range options {
		builder.WriteString(`<label class="inline-flex items-center gap-2 px-3 py-1 rounded-full border border-gray-300 bg-gray-100 text-gray-700 hover:bg-gray-200 transition" data-pill="true">`)
		builder.WriteString(`<input type="radio" class="sr-only" name="`)
		builder.WriteString(html.EscapeString(field.Name))
		builder.WriteString(`" value="`)
		builder.WriteString(html.EscapeString(option.Value))
		builder.WriteString(`"`)
		if idx == 0 && current == "" {
			builder.WriteString(` checked`)
		} else if option.Value == current {
			builder.WriteString(` checked`)
		}
		builder.WriteString(`>
            <span>`)
		builder.WriteString(html.EscapeString(option.Label))
		builder.WriteString(`</span>
          </label>`)
	}
	builder.WriteString(`</div>`)

	buf.WriteString(builder.String())
	return nil
}

func extractPillOptions(field model.Field, config map[string]any) []pillOption {
	if opts, ok := parseConfigOptions(config["options"]); ok && len(opts) > 0 {
		return opts
	}
	if len(field.Enum) == 0 {
		return nil
	}
	result := make([]pillOption, 0, len(field.Enum))
	for _, value := range field.Enum {
		str := toString(value)
		label := strings.Title(strings.ReplaceAll(str, "_", " "))
		result = append(result, pillOption{Label: label, Value: str})
	}
	return result
}

func parseConfigOptions(value any) ([]pillOption, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]pillOption, 0, len(list))
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		label := toString(obj["label"])
		if label == "" {
			continue
		}
		val := toString(obj["value"])
		if val == "" {
			val = label
		}
		result = append(result, pillOption{Label: label, Value: val})
	}
	return result, len(result) > 0
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func mustPreact() render.Renderer {
	r, err := preact.New(preact.WithAssetURLPrefix("/assets"))
	if err != nil {
		log.Fatalf("preact renderer: %v", err)
	}
	return r
}
