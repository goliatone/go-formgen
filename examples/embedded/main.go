// Command embedded is the Phase 1 integration example for the embedded-form
// primitives: compile a JSON Schema definition, render it with the vanilla
// renderer, and drive the browser controller through hydrate/edit/cancel,
// scoped validation, host-owned server issues, typed snapshots, and destroy.
//
// Everything that talks to a server lives in this example (the /api/validate
// route and adapter.js); the library itself adds no HTTP protocol, storage,
// telemetry, or CDN dependency. The definition is synthetic and contains no
// private data.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/goliatone/go-formgen"
	"github.com/goliatone/go-formgen/pkg/jsonschema"
	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	jsonrenderer "github.com/goliatone/go-formgen/pkg/renderers/json"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/submission"
)

//go:embed schema.json
var schemaJSON []byte

//go:embed adapter.js
var adapterJS []byte

const (
	schemaSourceName   = "schema.json"
	noteCounterName    = "note-counter"
	maxValidateBody    = 1 << 20
	defaultListenAddr  = "127.0.0.1:8089"
	descriptorScriptID = "formgen-descriptor"
)

// strictCSP forbids inline and remote scripts and styles: the page carries no
// inline JavaScript, the descriptor is a non-executable JSON data block, the
// vanilla stylesheet is served from the renderer's asset filesystem, and the
// vanilla grid layout travels as data attributes (data-formgen-columns,
// data-formgen-gutter, data-formgen-span, ...) that the served stylesheet
// matches, so `style-src 'self'` needs no 'unsafe-inline'.
const strictCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func main() {
	addr := flag.String("addr", defaultListenAddr, "address to listen on")
	renderOnly := flag.Bool("render", false, "print the rendered page to stdout and exit")
	csp := flag.Bool("csp", false, "send a restrictive Content-Security-Policy header with every response")
	flag.Parse()

	ctx := context.Background()
	server, err := newExampleServer(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "embedded example:", err)
		os.Exit(1)
	}
	server.csp = *csp

	if *renderOnly {
		if _, writeErr := os.Stdout.Write(server.page); writeErr != nil {
			fmt.Fprintln(os.Stderr, "embedded example:", writeErr)
			os.Exit(1)
		}
		return
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "embedded example:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "listening on http://%s\n", listener.Addr())
	if err := http.Serve(listener, server.routes()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "embedded example:", err)
		os.Exit(1)
	}
}

// exampleServer holds the compiled authoritative definition and the rendered page.
type exampleServer struct {
	form       model.FormModel
	page       []byte
	descriptor []byte
	csp        bool
}

func newExampleServer(ctx context.Context) (*exampleServer, error) {
	form, err := compileForm(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	formHTML, err := renderForm(ctx, form)
	if err != nil {
		return nil, fmt.Errorf("render form: %w", err)
	}
	descriptor, err := renderDescriptor(ctx, form)
	if err != nil {
		return nil, fmt.Errorf("render descriptor: %w", err)
	}
	return &exampleServer{
		form:       form,
		page:       composePage(formHTML, descriptor),
		descriptor: descriptor,
	}, nil
}

// compileForm normalizes the embedded JSON Schema through the canonical adapter
// and builds the complete form model, including the root condition set.
func compileForm(ctx context.Context) (model.FormModel, error) {
	document := jsonschema.MustNewDocument(jsonschema.SourceFromFS(schemaSourceName), schemaJSON)
	adapter := jsonschema.NewAdapter(memoryLoader{document: document})
	ir, err := adapter.Normalize(ctx, document, schema.NormalizeOptions{})
	if err != nil {
		return model.FormModel{}, err
	}
	refs, err := adapter.Forms(ctx, ir)
	if err != nil {
		return model.FormModel{}, err
	}
	if len(refs) == 0 {
		return model.FormModel{}, errors.New("schema produced no forms")
	}
	definition, ok := ir.Form(refs[0].ID)
	if !ok {
		return model.FormModel{}, fmt.Errorf("form %q missing from schema IR", refs[0].ID)
	}
	return model.NewBuilder().Build(definition)
}

// memoryLoader serves the single embedded document. The example schema has no
// $ref targets, so every load resolves to the same in-memory definition.
type memoryLoader struct {
	document schema.Document
}

func (l memoryLoader) Load(_ context.Context, _ jsonschema.Source) (schema.Document, error) {
	return l.document, nil
}

func renderForm(ctx context.Context, form model.FormModel) ([]byte, error) {
	registry := components.NewDefaultRegistry()
	if err := registry.Register(noteCounterName, components.Descriptor{Name: noteCounterName, Renderer: renderNoteCounter}); err != nil {
		return nil, err
	}
	renderer, err := vanilla.New(
		vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()),
		vanilla.WithStylesheet("/vanilla/formgen-vanilla.css"),
		vanilla.WithComponentRegistry(registry),
	)
	if err != nil {
		return nil, err
	}
	// Renderer-aware preflight: unknown or misconfigured components fail before
	// any page is served, with definition paths and causes only.
	if diagnostics := renderer.Preflight(form); len(diagnostics) > 0 {
		var causes []string
		for _, diagnostic := range diagnostics {
			causes = append(causes, fmt.Sprintf("%s at %s: %s", diagnostic.Code, diagnostic.Path, diagnostic.Message))
		}
		return nil, fmt.Errorf("component preflight failed: %s", strings.Join(causes, "; "))
	}
	return renderer.Render(ctx, form, render.RenderOptions{RenderMode: render.RenderModeForm})
}

func renderDescriptor(ctx context.Context, form model.FormModel) ([]byte, error) {
	return jsonrenderer.New().Render(ctx, form, render.RenderOptions{RenderMode: render.RenderModeFields})
}

// renderNoteCounter is the Go half of a registered extension control. It emits
// a plain text control plus a counter slot; adapter.js registers the browser
// behavior under the same name through registerComponent.
func renderNoteCounter(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
	controlID := strings.TrimSpace(field.Metadata["control.id"])
	if controlID == "" {
		controlID = "fg-" + field.Name
	}
	controlName := strings.TrimSpace(field.Metadata["control.name"])
	if controlName == "" {
		controlName = field.Name
	}
	var maxLength string
	for _, rule := range field.Validations {
		if rule.Kind == model.ValidationRuleMaxLength {
			maxLength = strings.TrimSpace(rule.Params["value"])
		}
	}
	value := ""
	if text, ok := field.Default.(string); ok {
		value = text
	}
	label := strings.TrimSpace(field.Label)
	if label == "" {
		label = field.Name
	}

	fmt.Fprintf(buf, `<div data-component=%q class="space-y-1">`, noteCounterName)
	fmt.Fprintf(buf, `<label for=%q class="block text-sm font-medium">%s</label>`, html.EscapeString(controlID), html.EscapeString(label))
	fmt.Fprintf(buf, `<input type="text" id=%q name=%q value=%q data-field-path=%q class="py-3 px-4 block w-full border border-gray-200 rounded-lg text-sm"`,
		html.EscapeString(controlID), html.EscapeString(controlName), html.EscapeString(value), html.EscapeString(field.Name))
	if maxLength != "" {
		fmt.Fprintf(buf, ` maxlength=%q`, html.EscapeString(maxLength))
	}
	if field.Required {
		buf.WriteString(" required")
	}
	if field.Metadata["validation.state"] == "invalid" {
		buf.WriteString(` aria-invalid="true"`)
	}
	buf.WriteString(">")
	buf.WriteString(`<span data-note-counter="true" class="text-xs text-gray-500" aria-live="polite"></span>`)
	buf.WriteString("</div>")
	return nil
}

func composePage(formHTML, descriptor []byte) []byte {
	// Keep the descriptor safe inside an inline script element.
	safeDescriptor := strings.ReplaceAll(string(descriptor), "</", `<\/`)
	var page bytes.Buffer
	page.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Embedded form primitives example</title>
<link rel="stylesheet" href="/example.css">
</head>
<body>
<main>
<section>
<h1>Embedded form primitives (Phase 1)</h1>
<p>Compile &rarr; render &rarr; hydrate/edit/cancel &rarr; scoped validation &rarr; server issues &rarr; typed snapshot &rarr; destroy. The runtime never fetches, submits, or stores anything; <code>adapter.js</code> owns transport, snapshots, and locking, while applicability comes from the shared <code>formgen.conditions/v1</code> definition.</p>
<div data-embedded-actions>
<button type="button" data-embedded-action="hydrate">Hydrate sample (silent)</button>
<button type="button" data-embedded-action="snapshot">Capture snapshot</button>
<button type="button" data-embedded-action="edit">Simulate edit</button>
<button type="button" data-embedded-action="cancel">Cancel (restore snapshot)</button>
<button type="button" data-embedded-action="reference">Restore reference</button>
<button type="button" data-embedded-action="validateColors">Validate colors only</button>
<button type="button" data-embedded-action="validateAll">Validate all</button>
<button type="button" data-embedded-action="server">Send to server</button>
<button type="button" data-embedded-action="clear">Clear issues</button>
<button type="button" data-embedded-action="lock">Capture and lock</button>
<button type="button" data-embedded-action="destroy">Destroy</button>
</div>
</section>
<section>
`)
	page.Write(formHTML)
	page.WriteString(`
</section>
<section>
<h2>Issues</h2>
<pre id="embedded-issues">[]</pre>
<h2>Snapshot</h2>
<pre id="embedded-snapshot">null</pre>
<h2>Log</h2>
<ul id="embedded-log"></ul>
</section>
</main>
<script type="application/json" id="` + descriptorScriptID + `">`)
	page.WriteString(safeDescriptor)
	page.WriteString(`</script>
<script src="/runtime/formgen-relationships.min.js" defer></script>
<script src="/adapter.js" defer></script>
</body>
</html>
`)
	return page.Bytes()
}

// exampleCSS styles the host page; it is served, never inlined, so the strict
// policy applies without 'unsafe-inline'.
const exampleCSS = `body { font-family: system-ui, sans-serif; margin: 0; padding: 1.5rem; color: #111827; background: #f9fafb; }
main { max-width: 64rem; margin: 0 auto; display: grid; gap: 1.5rem; }
section { background: #fff; border: 1px solid #e5e7eb; border-radius: 0.75rem; padding: 1rem 1.25rem; }
button { padding: 0.5rem 0.75rem; border: 1px solid #d1d5db; border-radius: 0.5rem; background: #fff; cursor: pointer; }
button:focus-visible, input:focus-visible, [data-formgen-form-errors]:focus-visible { outline: 3px solid #2563eb; outline-offset: 2px; }
[data-embedded-actions] { display: flex; flex-wrap: wrap; gap: 0.5rem; }
pre { background: #111827; color: #f9fafb; padding: 0.75rem; border-radius: 0.5rem; overflow: auto; font-size: 0.8rem; }
ul#embedded-log { max-height: 14rem; overflow: auto; font-size: 0.85rem; padding-left: 1.25rem; }
[data-formgen-form-errors] { border: 1px solid #dc2626; color: #991b1b; border-radius: 0.5rem; padding: 0.75rem; }
[data-formgen-group-invalid] { outline: 2px dashed #dc2626; outline-offset: 4px; }
`

func (s *exampleServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/runtime/", http.StripPrefix("/runtime/", http.FileServerFS(formgen.RuntimeAssetsFS())))
	mux.Handle("/vanilla/", http.StripPrefix("/vanilla/", http.FileServerFS(vanilla.AssetsFS())))
	mux.HandleFunc("/example.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(exampleCSS))
	})
	mux.HandleFunc("/adapter.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(adapterJS)
	})
	mux.HandleFunc("/descriptor.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(s.descriptor)
	})
	mux.HandleFunc("/api/validate", s.handleValidate)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(s.page)
	})
	if !s.csp {
		return mux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", strictCSP)
		mux.ServeHTTP(w, r)
	})
}

// wireIssue is the browser-facing issue shape: codes, paths, and messages only.
// Submitted values never travel back to the page or into logs.
type wireIssue struct {
	Code         string `json:"code"`
	Path         string `json:"path,omitempty"`
	RendererPath string `json:"rendererPath,omitempty"`
	Message      string `json:"message"`
}

type validateResponse struct {
	Issues     []wireIssue     `json:"issues"`
	Applicable map[string]bool `json:"applicable,omitempty"`
}

// handleValidate parses the normalized snapshot against the complete
// authoritative definition, applies the shared conditions with the reject
// policy for supplied inactive inputs, and returns every parse, condition, and
// validation issue. It persists nothing.
func (s *exampleServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxValidateBody))
	if err != nil {
		http.Error(w, "request body too large or unreadable", http.StatusBadRequest)
		return
	}
	parsed, err := submission.ParseJSON(s.form, body)
	if err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	// A body that is not a JSON object cannot be validated against the
	// definition; report only the parse issue instead of every required field.
	if bodyIssues := bodyLevelIssues(parsed.Issues); len(bodyIssues) > 0 {
		writeIssues(w, http.StatusBadRequest, validateResponse{Issues: bodyIssues})
		return
	}
	result := submission.ValidateConditional(s.form, parsed, submission.InactiveReject)
	writeIssues(w, http.StatusOK, validateResponse{Issues: toWire(result.Issues), Applicable: result.Applicable})
}

func bodyLevelIssues(issues []submission.Issue) []wireIssue {
	var out []wireIssue
	for _, issue := range issues {
		if issue.Path == "" && (issue.Code == submission.CodeInvalidJSON || issue.Code == submission.CodeType) {
			out = append(out, toWire([]submission.Issue{issue})...)
		}
	}
	return out
}

func toWire(issues []submission.Issue) []wireIssue {
	out := make([]wireIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, wireIssue{
			Code:         string(issue.Code),
			Path:         issue.Path,
			RendererPath: issue.RendererPath,
			Message:      issue.Message,
		})
	}
	return out
}

func writeIssues(w http.ResponseWriter, status int, response validateResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
	}
}
