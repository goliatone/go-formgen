package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing/fstest"
	"time"

	"github.com/goliatone/go-formgen"
	"github.com/goliatone/go-formgen/examples/internal/exampleutil"
	"github.com/goliatone/go-formgen/pkg/model"
	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/orchestrator"
	"github.com/goliatone/go-formgen/pkg/render"
	gotemplate "github.com/goliatone/go-formgen/pkg/render/template/gotemplate"
	"github.com/goliatone/go-formgen/pkg/renderers/preact"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
)

const vanillaRuntimeBootstrap = `
<script src="/runtime/formgen-relationships.min.js" defer></script>
<script src="/runtime/formgen-behaviors.min.js" defer></script>
<script>
function buildCreateModalRegistry() {
  var registry = {};
  var nodes = document.querySelectorAll('[data-fg-create-modal]');
  Array.prototype.forEach.call(nodes, function (modal) {
    var actionId = modal.getAttribute('data-fg-create-modal');
    if (!actionId) {
      return;
    }
    var form = modal.querySelector('form');
    if (!form) {
      return;
    }
    var labelField = modal.getAttribute('data-fg-create-label-field') || 'name';
    var valueField = modal.getAttribute('data-fg-create-value-field') || 'id';
    var prefillField = modal.getAttribute('data-fg-create-prefill-field') || labelField;
    registry[actionId] = {
      element: modal,
      form: form,
      labelField: labelField,
      valueField: valueField,
      prefillField: prefillField,
    };
  });
  return registry;
}

function setModalOpen(spec, open) {
  if (!spec || !spec.element) {
    return;
  }
  spec.element.hidden = !open;
  document.body.classList.toggle('overflow-hidden', open);
}

function prefillModal(spec, query) {
  if (!spec || !spec.form) {
    return;
  }
  var fieldName = spec.prefillField;
  if (!fieldName) {
    return;
  }
  // Only prefill if we have a non-empty query; otherwise preserve defaults
  var trimmed = (query && typeof query === 'string') ? query.trim() : '';
  if (!trimmed) {
    return;
  }
  var input = spec.form.querySelector('[name="' + fieldName + '"]');
  if (!input) {
    return;
  }
  input.value = trimmed;
  try {
    input.dispatchEvent(new Event('input', { bubbles: true }));
  } catch (_err) {
    // Ignore event errors in older browsers.
  }
}

function formDataToObject(form) {
  var payload = {};
  var data = new FormData(form);
  data.forEach(function (value, key) {
    if (payload[key] === undefined) {
      payload[key] = value;
      return;
    }
    if (Array.isArray(payload[key])) {
      payload[key].push(value);
      return;
    }
    payload[key] = [payload[key], value];
  });
  var checkboxes = form.querySelectorAll('input[type="checkbox"][name]');
  Array.prototype.forEach.call(checkboxes, function (input) {
    if (payload[input.name] === undefined) {
      payload[input.name] = input.checked;
    } else if (payload[input.name] === 'on') {
      payload[input.name] = input.checked;
    }
  });
  return payload;
}

function normalizeCreatePayload(payload) {
  if (payload && typeof payload === 'object' && payload.data) {
    return payload.data;
  }
  return payload;
}

function buildCreatedOption(spec, payload) {
  var record = normalizeCreatePayload(payload);
  if (!record || typeof record !== 'object') {
    return null;
  }
  var value = record[spec.valueField];
  var label = record[spec.labelField];
  if (value === undefined || label === undefined) {
    return null;
  }
  return {
    value: String(value),
    label: String(label),
  };
}

function submitCreateForm(spec) {
  var form = spec.form;
  var action = form.getAttribute('action') || window.location.href;
  var method = (form.getAttribute('method') || 'POST').toUpperCase();
  var payload = formDataToObject(form);
  return fetch(action, {
    method: method,
    headers: {
      'Accept': 'application/json',
      'Content-Type': 'application/json',
    },
    credentials: 'same-origin',
    body: JSON.stringify(payload),
  }).then(function (response) {
    if (response.status === 204) {
      return {};
    }
    return response.json().then(function (data) {
      if (!response.ok) {
        var message = (data && data.error) ? data.error : response.statusText;
        throw new Error(message);
      }
      return data;
    }).catch(function (err) {
      if (!response.ok) {
        throw err;
      }
      return {};
    });
  });
}

function getFocusableElements(container) {
  var selector = 'button, [href], input:not([type="hidden"]), select, textarea, [tabindex]:not([tabindex="-1"])';
  var elements = container.querySelectorAll(selector);
  return Array.prototype.filter.call(elements, function (el) {
    return !el.disabled && el.offsetParent !== null;
  });
}

function openCreateModal(spec, query) {
  if (!spec || !spec.element || !spec.form) {
    return Promise.resolve(null);
  }
  if (!spec.element.hidden) {
    return Promise.resolve(null);
  }
  setModalOpen(spec, true);
  prefillModal(spec, query);
  var focusable = spec.form.querySelector('input:not([type="hidden"]), select, textarea, button');
  if (focusable) {
    focusable.focus();
  }

  return new Promise(function (resolve) {
    var handled = false;
    var overlay = spec.element.querySelector('[data-fg-modal-overlay]');
    var closeButtons = spec.element.querySelectorAll('[data-fg-modal-close], button[type="button"]');

    function cleanup() {
      if (overlay) {
        overlay.removeEventListener('click', onCancel);
      }
      Array.prototype.forEach.call(closeButtons, function (button) {
        button.removeEventListener('click', onCancel);
      });
      spec.form.removeEventListener('submit', onSubmit);
      document.removeEventListener('keydown', onKeyDown);
    }

    function close(result) {
      if (handled) {
        return;
      }
      handled = true;
      cleanup();
      setModalOpen(spec, false);
      // Reset form when closing modal
      if (spec.form && typeof spec.form.reset === 'function') {
        spec.form.reset();
      }
      // Clear any error messages
      var errorContainer = spec.element.querySelector('[data-fg-modal-error]');
      if (errorContainer) {
        errorContainer.textContent = '';
        errorContainer.hidden = true;
      }
      resolve(result || null);
    }

    function onCancel(event) {
      if (event) {
        event.preventDefault();
      }
      close(null);
    }

    function onKeyDown(event) {
      if (event.key === 'Escape') {
        event.preventDefault();
        close(null);
        return;
      }
      // Focus trap: keep Tab cycling within the modal
      if (event.key === 'Tab') {
        var focusableEls = getFocusableElements(spec.element);
        if (focusableEls.length === 0) {
          event.preventDefault();
          return;
        }
        var firstEl = focusableEls[0];
        var lastEl = focusableEls[focusableEls.length - 1];
        if (event.shiftKey) {
          // Shift+Tab: if on first element, wrap to last
          if (document.activeElement === firstEl) {
            event.preventDefault();
            lastEl.focus();
          }
        } else {
          // Tab: if on last element, wrap to first
          if (document.activeElement === lastEl) {
            event.preventDefault();
            firstEl.focus();
          }
        }
      }
    }

    function showError(message) {
      var errorContainer = spec.element.querySelector('[data-fg-modal-error]');
      if (errorContainer) {
        errorContainer.textContent = message || 'An error occurred. Please try again.';
        errorContainer.hidden = false;
      }
    }

    function onSubmit(event) {
      event.preventDefault();
      // Disable submit button and show loading state
      var submitBtn = spec.form.querySelector('button[type="submit"]');
      var originalText = '';
      if (submitBtn) {
        originalText = submitBtn.textContent;
        submitBtn.disabled = true;
        submitBtn.textContent = 'Creating...';
      }
      // Clear previous errors
      var errorContainer = spec.element.querySelector('[data-fg-modal-error]');
      if (errorContainer) {
        errorContainer.hidden = true;
      }
      submitCreateForm(spec)
        .then(function (payload) {
          close(buildCreatedOption(spec, payload));
        })
        .catch(function (error) {
          console.warn('[formgen] create action failed', error);
          showError(error.message || 'Failed to create record.');
        })
        .finally(function () {
          // Restore submit button state
          if (submitBtn) {
            submitBtn.disabled = false;
            submitBtn.textContent = originalText;
          }
        });
    }

    if (overlay) {
      overlay.addEventListener('click', onCancel);
    }
    Array.prototype.forEach.call(closeButtons, function (button) {
      button.addEventListener('click', onCancel);
    });
    spec.form.addEventListener('submit', onSubmit);
    document.addEventListener('keydown', onKeyDown);
  });
}

window.addEventListener('DOMContentLoaded', function () {
  if (!window.FormgenRelationships || typeof window.FormgenRelationships.initRelationships !== 'function') {
    console.warn('formgen runtime bundle not loaded; ensure /runtime/ is served from formgen.RuntimeAssetsFS()');
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
  var delayButton = document.getElementById('apply-delay-button');
  var delayStatus = document.getElementById('apply-delay-status');
  var relationshipRegistry = null;
  var pendingDelay = null;

  function setDelayStatus(message) {
    if (!delayStatus) {
      return;
    }
    delayStatus.textContent = message || '';
  }

  function randomDelayValue() {
    var delay = 1.5 + (Math.random() * 1.5);
    return delay.toFixed(1) + 's';
  }

  function applyDelayToElements(delayValue) {
    var nodes = document.querySelectorAll('[data-endpoint-url]');
    if (!nodes || nodes.length === 0) {
      setDelayStatus('No relationship fields found.');
      return false;
    }
    Array.prototype.forEach.call(nodes, function (element) {
      element.setAttribute('data-endpoint-params-_delay', delayValue);
    });
    return true;
  }

  function applyDelayToResolvers(delayValue) {
    if (!relationshipRegistry || typeof relationshipRegistry.get !== 'function') {
      return;
    }
    var nodes = document.querySelectorAll('[data-endpoint-url]');
    Array.prototype.forEach.call(nodes, function (element) {
      var resolver = relationshipRegistry.get(element);
      if (!resolver || !resolver.endpoint) {
        return;
      }
      if (!resolver.endpoint.params) {
        resolver.endpoint.params = {};
      }
      resolver.endpoint.params['_delay'] = delayValue;
    });
  }

  function applyRelationshipDelay() {
    var delayValue = randomDelayValue();
    var updated = applyDelayToElements(delayValue);
    if (!updated) {
      return;
    }
    if (relationshipRegistry) {
      applyDelayToResolvers(delayValue);
    } else {
      pendingDelay = delayValue;
    }
    setDelayStatus('Delay set to ' + delayValue + ' for all relationships.');
  }

  if (delayButton) {
    delayButton.addEventListener('click', applyRelationshipDelay);
  }
  var modalRegistry = buildCreateModalRegistry();
  var config = {
    onValidationError: function (ctx, error) {
      console.warn('[formgen] validation failed', ctx.field.name, error.message);
    },
    cache: { strategy: 'none' },
  };
  if (Object.keys(modalRegistry).length > 0) {
    config.onCreateAction = function (_context, detail) {
      if (!detail || !detail.actionId) {
        return;
      }
      var spec = modalRegistry[detail.actionId];
      if (!spec) {
        return;
      }
      return openCreateModal(spec, detail.query || '');
    };
  }
  window.FormgenRelationships.initRelationships(config).then(function (registry) {
    relationshipRegistry = registry;
    if (pendingDelay) {
      applyDelayToResolvers(pendingDelay);
      pendingDelay = null;
    }
  }).catch(function (error) {
    console.error('initRelationships failed', error);
  });
  if (window.FormgenBehaviors && typeof window.FormgenBehaviors.initBehaviors === 'function') {
    window.FormgenBehaviors.initBehaviors();
  } else {
    console.warn('formgen behaviors bundle not loaded; ensure /runtime/ is served from formgen.RuntimeAssetsFS()');
  }
});
</script>
`

const advancedRendererName = "vanilla"
const advancedMainOperation = "post-book:create"

type createModalSpec struct {
	ActionID     string
	OperationID  string
	ValueField   string
	LabelField   string
	PrefillField string
}

var advancedCreateModals = []createModalSpec{
	{
		ActionID:     "author",
		OperationID:  "post-author:create",
		ValueField:   "id",
		LabelField:   "full_name",
		PrefillField: "full_name",
	},
	{
		ActionID:     "publisher",
		OperationID:  "post-publishing-house:create",
		ValueField:   "id",
		LabelField:   "name",
		PrefillField: "name",
	},
	{
		ActionID:     "tag",
		OperationID:  "post-tag:create",
		ValueField:   "id",
		LabelField:   "name",
		PrefillField: "name",
	},
}

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

func defaultTemplatesDir(schemaPath string) string {
	path := strings.TrimSpace(schemaPath)
	if path == "" {
		return ""
	}
	candidate := filepath.Clean(filepath.Join(filepath.Dir(path), "templates"))
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return ""
	}
	return candidate
}

func resolveUISchemaFS(source string) (fs.FS, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, nil
	}
	if isURL(trimmed) {
		resp, err := http.Get(trimmed)
		if err != nil {
			return nil, fmt.Errorf("fetch ui schema %q: %w", trimmed, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("fetch ui schema %q: status %d", trimmed, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read ui schema %q: %w", trimmed, err)
		}
		name := uiSchemaFilenameFromURL(trimmed, resp.Header.Get("Content-Type"))
		return fstest.MapFS{
			name: &fstest.MapFile{Data: data},
		}, nil
	}

	info, err := os.Stat(trimmed)
	if err != nil {
		return nil, fmt.Errorf("ui schema %q: %w", trimmed, err)
	}
	if info.IsDir() {
		return os.DirFS(trimmed), nil
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	if ext != ".json" && ext != ".yaml" && ext != ".yml" {
		return nil, fmt.Errorf("ui schema %q: unsupported extension %q", trimmed, ext)
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, fmt.Errorf("read ui schema %q: %w", trimmed, err)
	}
	return fstest.MapFS{
		filepath.Base(trimmed): &fstest.MapFile{Data: data},
	}, nil
}

func uiSchemaFilenameFromURL(raw string, contentType string) string {
	parsed, err := url.Parse(raw)
	if err == nil {
		base := path.Base(parsed.Path)
		ext := strings.ToLower(path.Ext(base))
		if ext == ".json" || ext == ".yaml" || ext == ".yml" {
			return base
		}
	}
	if strings.Contains(strings.ToLower(contentType), "yaml") {
		return "schema.yaml"
	}
	return "schema.json"
}

func isURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func main() {
	defaultSource := defaultSchemaPath()

	var (
		addrFlag      = flag.String("addr", ":8383", "HTTP listen address")
		sourceFlag    = flag.String("source", defaultSource, "Default OpenAPI source (file path or URL)")
		uiSchemaFlag  = flag.String("ui", "", "UI schema directory or file (path or URL)")
		templatesFlag = flag.String("templates", "", "Templates directory for advanced view (local path)")
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

	uiSchemaSource := strings.TrimSpace(*uiSchemaFlag)
	if uiSchemaSource == "" {
		uiSchemaSource = defaultUISchemaDir(*sourceFlag)
	}

	templatesDir := strings.TrimSpace(*templatesFlag)
	if templatesDir == "" {
		templatesDir = defaultTemplatesDir(*sourceFlag)
	}

	options := []orchestrator.Option{
		orchestrator.WithLoader(loader),
		orchestrator.WithParser(parser),
		orchestrator.WithModelBuilder(builder),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(*rendererFlag),
	}
	if uiSchemaSource != "" {
		uiSchemaFS, err := resolveUISchemaFS(uiSchemaSource)
		if err != nil {
			log.Fatalf("ui schema: %v", err)
		}
		if uiSchemaFS != nil {
			options = append(options, orchestrator.WithUISchemaFS(uiSchemaFS))
		}
	}

	var templateEngine *gotemplate.Engine
	if templatesDir != "" {
		if isURL(templatesDir) {
			log.Fatalf("templates: expected local directory, got URL %q", templatesDir)
		}
		info, err := os.Stat(templatesDir)
		if err != nil {
			log.Fatalf("templates: %v", err)
		}
		if !info.IsDir() {
			log.Fatalf("templates: %q is not a directory", templatesDir)
		}
		engine, err := gotemplate.New(
			gotemplate.WithBaseDir(templatesDir),
			gotemplate.WithExtension(".tmpl"),
		)
		if err != nil {
			log.Fatalf("templates: %v", err)
		}
		templateEngine = engine
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
		templates:        templateEngine,
		cache:            newDocumentCache(),
		defaultSource:    *sourceFlag,
		defaultRenderer:  *rendererFlag,
		defaultOperation: defaultOperation,
	}

	relationships := newRelationshipDataset()

	mux := http.NewServeMux()
	relationships.register(mux)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(preact.AssetsFS()))))
	mux.Handle("/runtime/", http.StripPrefix("/runtime/", http.FileServerFS(formgen.RuntimeAssetsFS())))
	mux.HandleFunc("/api/uploads/", uploadHandler)
	mux.Handle("/form", server.formHandler())
	mux.Handle("/advanced", server.advancedHandler())
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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart payload", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := filepath.Base(header.Filename)
	payload := map[string]any{
		"url":          fmt.Sprintf("/uploads/%d_%s", time.Now().Unix(), filename),
		"name":         filename,
		"originalName": filename,
		"size":         header.Size,
		"contentType":  header.Header.Get("Content-Type"),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("upload encode: %v", err)
	}
}

type formServer struct {
	generator        *orchestrator.Orchestrator
	loader           pkgopenapi.Loader
	parser           pkgopenapi.Parser
	builder          model.Builder
	registry         *render.Registry
	templates        *gotemplate.Engine
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

		renderOptions := render.RenderOptions{}
		sampleKey := strings.TrimSpace(query.Get("id"))
		if sampleKey == "" {
			sampleKey = strings.TrimSpace(query.Get("record"))
		}
		if sampleKey != "" {
			if sample, ok := sampleRenderOptionsFor(sampleKey); ok {
				renderOptions = sample
			}
		}
		if methodOverride := strings.TrimSpace(query.Get("method")); methodOverride != "" {
			renderOptions.Method = methodOverride
		}

		output, err := s.generator.Generate(r.Context(), orchestrator.Request{
			Document:      &document,
			OperationID:   operation,
			Renderer:      rendererName,
			RenderOptions: renderOptions,
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

func (s *formServer) advancedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.templates == nil {
			http.Error(w, "templates not configured", http.StatusInternalServerError)
			return
		}
		if !s.registry.Has(advancedRendererName) {
			http.Error(w, fmt.Sprintf("renderer %q not found", advancedRendererName), http.StatusNotFound)
			return
		}

		sourceRaw := strings.TrimSpace(r.URL.Query().Get("source"))
		if sourceRaw == "" {
			sourceRaw = s.defaultSource
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

		mainOutput, err := s.generator.Generate(r.Context(), orchestrator.Request{
			Document:    &document,
			OperationID: advancedMainOperation,
			Renderer:    advancedRendererName,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("generate main form: %v", err), http.StatusInternalServerError)
			return
		}

		modals := make([]map[string]any, 0, len(advancedCreateModals))
		for _, spec := range advancedCreateModals {
			modalOutput, err := s.generator.Generate(r.Context(), orchestrator.Request{
				Document:    &document,
				OperationID: spec.OperationID,
				Renderer:    advancedRendererName,
				RenderOptions: render.RenderOptions{
					Subset: render.FieldSubset{
						Tags: []string{"modal-min"},
					},
					OmitAssets: true, // Modals inherit assets from the parent page
				},
			})
			if err != nil {
				http.Error(w, fmt.Sprintf("generate modal form (%s): %v", spec.OperationID, err), http.StatusInternalServerError)
				return
			}
			modals = append(modals, map[string]any{
				"action_id":     spec.ActionID,
				"value_field":   spec.ValueField,
				"label_field":   spec.LabelField,
				"prefill_field": spec.PrefillField,
				"form_html":     string(modalOutput),
			})
		}

		page, err := s.templates.RenderTemplate("advanced", map[string]any{
			"main_form_html":    string(mainOutput),
			"modals":            modals,
			"runtime_bootstrap": vanillaRuntimeBootstrap,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("render template: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write([]byte(page)); err != nil {
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
	mu             sync.RWMutex
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
				ID:          "11111111-1111-4111-8111-111111111111",
				FullName:    "Ada Lovelace",
				Email:       "ada@example.com",
				PublisherID: "cccc3333-cccc-4333-8333-cccccccccccc", // Northwind Publishing
				Active:      true,
				TenantID:    "northwind",
				Profile: authorProfile{
					Bio:     "First programmer and resident polymath.",
					Twitter: "@ada",
				},
			},
			{
				ID:          "22222222-2222-4222-8222-222222222222",
				FullName:    "Claude Shannon",
				Email:       "claude@example.com",
				PublisherID: "cccc3333-cccc-4333-8333-cccccccccccc", // Northwind Publishing
				Active:      true,
				TenantID:    "northwind",
				Profile: authorProfile{
					Bio:     "Father of information theory.",
					Twitter: "@entropy",
				},
			},
			{
				ID:          "33333333-3333-4333-8333-333333333333",
				FullName:    "Octavia E. Butler",
				Email:       "octavia@example.com",
				PublisherID: "bbbb2222-bbbb-4222-8222-bbbbbbbbbbbb", // Lumen House
				Active:      true,
				TenantID:    "lumen",
				Profile: authorProfile{
					Bio:     "Visionary science fiction author.",
					Twitter: "@patternist",
				},
			},
			{
				ID:          "44444444-4444-4444-8444-444444444444",
				FullName:    "Ida B. Wells",
				Email:       "ida@example.com",
				PublisherID: "bbbb2222-bbbb-4222-8222-bbbbbbbbbbbb", // Lumen House
				Active:      true,
				TenantID:    "lumen",
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
			{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Name: "feature", Category: "product", Description: "Feature launch related content"},
			{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Name: "announcement", Category: "company", Description: "Company announcements"},
			{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Name: "editorial", Category: "editorial", Description: "Long-form editorial work"},
			{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", Name: "people", Category: "culture", Description: "People and culture stories"},
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
	mux.HandleFunc("/author", d.handleAuthorCreate)
	mux.HandleFunc("/api/authors", d.handleAuthors)
	mux.HandleFunc("/api/categories", d.handleCategories)
	mux.HandleFunc("/api/managers", d.handleManagers)
	mux.HandleFunc("/tag", d.handleTagCreate)
	mux.HandleFunc("/api/tags", d.handleTags)
	mux.HandleFunc("/publishing-house", d.handlePublishingHouseCreate)
	mux.HandleFunc("/api/publishing-houses", d.handlePublishingHouses)
	mux.HandleFunc("/api/author-profiles", d.handleAuthorProfiles)
	mux.HandleFunc("/api/books", d.handleBooks)
	mux.HandleFunc("/api/chapters", d.handleChapters)
	mux.HandleFunc("/api/headquarters", d.handleHeadquarters)
}

func (d *relationshipDataset) handleAuthors(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		d.handleAuthorCreate(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowedWith(w, http.MethodGet, http.MethodPost)
		return
	}

	// Apply artificial delay to demonstrate loading indicators.
	// Example: /api/authors?_delay=1s
	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

func (d *relationshipDataset) handleAuthorCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowedWith(w, http.MethodPost)
		return
	}

	payload, err := parseCreatePayload(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	fullName := payloadString(payload, "full_name")
	email := payloadString(payload, "email")
	publisherID := payloadString(payload, "publisher_id")
	if fullName == "" || email == "" || publisherID == "" {
		http.Error(w, "full_name, email, and publisher_id are required", http.StatusBadRequest)
		return
	}

	record := authorRecord{
		ID:          newRecordID("author"),
		FullName:    fullName,
		Email:       email,
		PublisherID: publisherID,
		Active:      payloadBool(payload, "active"),
		TenantID:    "northwind",
	}

	d.mu.Lock()
	d.authors = append(d.authors, record)
	d.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(record); err != nil {
		log.Printf("write author create response: %v", err)
	}
}

func (d *relationshipDataset) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, d.managers, func(m managerRecord) string { return m.ID }, func(m managerRecord) string {
			return m.Name
		})
		return
	}

	writeRelationshipData(w, d.managers)
}

func (d *relationshipDataset) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		d.handleTagCreate(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowedWith(w, http.MethodGet, http.MethodPost)
		return
	}

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	results := make([]tagRecord, 0, len(d.tags))
	for _, tag := range d.tags {
		if search != "" && !strings.Contains(strings.ToLower(tag.Name), search) {
			continue
		}
		results = append(results, tag)
	}

	if wantsOptionsFormat(r) {
		writeRelationshipOptions(w, results, func(t tagRecord) string { return t.ID }, func(t tagRecord) string {
			return t.Name
		})
		return
	}

	writeRelationshipData(w, results)
}

func (d *relationshipDataset) handleTagCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowedWith(w, http.MethodPost)
		return
	}

	payload, err := parseCreatePayload(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	name := payloadString(payload, "name")
	category := payloadString(payload, "category")
	if name == "" || category == "" {
		http.Error(w, "name and category are required", http.StatusBadRequest)
		return
	}

	record := tagRecord{
		ID:          newRecordID("tag"),
		Name:        name,
		Category:    category,
		Description: payloadString(payload, "description"),
	}

	d.mu.Lock()
	d.tags = append(d.tags, record)
	d.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(record); err != nil {
		log.Printf("write tag create response: %v", err)
	}
}

func (d *relationshipDataset) handlePublishingHouses(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		d.handlePublishingHouseCreate(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowedWith(w, http.MethodGet, http.MethodPost)
		return
	}

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

func (d *relationshipDataset) handlePublishingHouseCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowedWith(w, http.MethodPost)
		return
	}

	payload, err := parseCreatePayload(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	name := payloadString(payload, "name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	record := publishingHouseRecord{
		ID:            newRecordID("publisher"),
		Name:          name,
		ImprintPrefix: payloadString(payload, "imprint_prefix"),
	}

	d.mu.Lock()
	d.publishers = append(d.publishers, record)
	d.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(record); err != nil {
		log.Printf("write publishing house create response: %v", err)
	}
}

func (d *relationshipDataset) handleAuthorProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

	applyArtificialDelay(r)

	d.mu.RLock()
	defer d.mu.RUnlock()

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

func parseCreatePayload(r *http.Request) (map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("request is nil")
	}
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "application/json") {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		return payload, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	payload := make(map[string]any, len(r.Form))
	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}
		if len(values) == 1 {
			payload[key] = values[0]
			continue
		}
		payload[key] = values
	}
	return payload, nil
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		if len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
	case []any:
		if len(v) > 0 {
			return strings.TrimSpace(fmt.Sprintf("%v", v[0]))
		}
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

func payloadBool(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		normalized := strings.ToLower(strings.TrimSpace(v))
		return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "on"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func newRecordID(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = "record"
	}
	return fmt.Sprintf("%s-%d", trimmed, time.Now().UnixNano())
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

func methodNotAllowedWith(w http.ResponseWriter, allowed ...string) {
	if len(allowed) == 0 {
		methodNotAllowed(w)
		return
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// applyArtificialDelay reads an optional "_delay" query parameter and sleeps
// for that duration. Use this to demonstrate loading indicators.
// Examples: ?_delay=500ms, ?_delay=1s, ?_delay=2s
func applyArtificialDelay(r *http.Request) {
	delayStr := r.URL.Query().Get("_delay")
	if delayStr == "" {
		return
	}
	d, err := time.ParseDuration(delayStr)
	if err != nil || d <= 0 || d > 10*time.Second {
		return // Ignore invalid or excessive delays
	}
	time.Sleep(d)
}

type authorRecord struct {
	ID          string        `json:"id"`
	FullName    string        `json:"full_name"`
	Email       string        `json:"email"`
	PublisherID string        `json:"publisher_id"`
	Active      bool          `json:"active"`
	TenantID    string        `json:"tenant_id"`
	Profile     authorProfile `json:"profile"`
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
	Name        string `json:"name"`
	Category    string `json:"category"`
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
