package orchestrator_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestWithEndpointOverrides_AppliesMetadata(t *testing.T) {
	t.Parallel()

	ctx := testsupport.Context()
	source := pkgopenapi.SourceFromFile(filepath.Join("testdata", "relationships.yaml"))

	registry := render.NewRegistry()
	registry.MustRegister(&captureRenderer{})

	override := orchestrator.EndpointOverride{
		OperationID: "createArticle",
		FieldPath:   "article.author_id",
		Endpoint: orchestrator.EndpointConfig{
			URL:        "/api/authors",
			Method:     "get",
			LabelField: "full_name",
			ValueField: "id",
			Params: map[string]string{
				"include": "profile",
			},
			DynamicParams: map[string]string{
				"tenant_id": "{{field:tenant_id}}",
			},
			Auth: &orchestrator.EndpointAuth{
				Strategy: "header",
				Header:   "X-Auth-Token",
				Source:   "meta:formgen-auth",
			},
		},
	}

	orch := orchestrator.New(
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer("capture"),
		orchestrator.WithUISchemaFS(nil),
		orchestrator.WithEndpointOverrides([]orchestrator.EndpointOverride{override}),
	)

	output, err := orch.Generate(ctx, orchestrator.Request{
		Source:      source,
		OperationID: "createArticle",
	})
	if err != nil {
		t.Fatalf("generate form: %v", err)
	}

	var form pkgmodel.FormModel
	if err := json.Unmarshal(output, &form); err != nil {
		t.Fatalf("unmarshal form model: %v", err)
	}

	field := findField(form.Fields, []string{"article", "author_id"})
	if field == nil {
		t.Fatalf("author_id field not found")
	}
	if field.Metadata == nil {
		t.Fatalf("metadata not initialised")
	}

	if got := field.Metadata["relationship.endpoint.url"]; got != "/api/authors" {
		t.Fatalf("relationship.endpoint.url = %q, want %q", got, "/api/authors")
	}
	if got := field.Metadata["relationship.endpoint.method"]; got != "GET" {
		t.Fatalf("relationship.endpoint.method = %q, want %q", got, "GET")
	}
	if got := field.Metadata["relationship.endpoint.labelField"]; got != "full_name" {
		t.Fatalf("relationship.endpoint.labelField = %q, want %q", got, "full_name")
	}
	if got := field.Metadata["relationship.endpoint.valueField"]; got != "id" {
		t.Fatalf("relationship.endpoint.valueField = %q, want %q", got, "id")
	}
	if got := field.Metadata["relationship.endpoint.params.include"]; got != "profile" {
		t.Fatalf("relationship.endpoint.params.include = %q, want %q", got, "profile")
	}
	if got := field.Metadata["relationship.endpoint.dynamicParams.tenant_id"]; got != "{{field:tenant_id}}" {
		t.Fatalf("relationship.endpoint.dynamicParams.tenant_id = %q, want %q", got, "{{field:tenant_id}}")
	}
	if got := field.Metadata["relationship.endpoint.refreshOn"]; got != "tenant_id" {
		t.Fatalf("relationship.endpoint.refreshOn = %q, want %q", got, "tenant_id")
	}
	if got := field.Metadata["relationship.endpoint.auth.strategy"]; got != "header" {
		t.Fatalf("relationship.endpoint.auth.strategy = %q, want %q", got, "header")
	}
	if got := field.Metadata["relationship.endpoint.auth.header"]; got != "X-Auth-Token" {
		t.Fatalf("relationship.endpoint.auth.header = %q, want %q", got, "X-Auth-Token")
	}
	if got := field.Metadata["relationship.endpoint.auth.source"]; got != "meta:formgen-auth" {
		t.Fatalf("relationship.endpoint.auth.source = %q, want %q", got, "meta:formgen-auth")
	}

	// Ensure unrelated fields remain untouched.
	if other := findField(form.Fields, []string{"article"}); other != nil && other != field {
		if other.Metadata != nil {
			if _, ok := other.Metadata["relationship.endpoint.url"]; ok {
				t.Fatalf("unexpected endpoint metadata on parent article field")
			}
		}
	}
}

func TestWithEndpointOverrides_RenderedAttributes(t *testing.T) {
	ctx := testsupport.Context()
	source := pkgopenapi.SourceFromFile(filepath.Join("testdata", "relationships.yaml"))

	override := orchestrator.EndpointOverride{
		OperationID: "createArticle",
		FieldPath:   "article.author_id",
		Endpoint: orchestrator.EndpointConfig{
			URL:        "/api/authors",
			Method:     "GET",
			LabelField: "full_name",
			ValueField: "id",
			DynamicParams: map[string]string{
				"tenant_id": "{{field:tenant_id}}",
			},
			Auth: &orchestrator.EndpointAuth{
				Strategy: "header",
				Header:   "X-Auth-Token",
				Source:   "meta:formgen-auth",
			},
		},
	}

	orch := orchestrator.New(
		orchestrator.WithRegistry(defaultVanillaRegistry(t)),
		orchestrator.WithUISchemaFS(nil),
		orchestrator.WithEndpointOverrides([]orchestrator.EndpointOverride{override}),
	)

	output, err := orch.Generate(ctx, orchestrator.Request{
		Source:      source,
		OperationID: "createArticle",
		Renderer:    "vanilla",
	})
	if err != nil {
		t.Fatalf("generate html: %v", err)
	}

	html := string(output)
	assertContains(t, html, `data-endpoint-url="/api/authors"`)
	assertContains(t, html, `data-endpoint-method="GET"`)
	assertContains(t, html, `data-endpoint-label-field="full_name"`)
	assertContains(t, html, `data-endpoint-value-field="id"`)
	assertContains(t, html, `data-endpoint-refresh-on="tenant_id"`)
	assertContains(t, html, `data-auth-source="meta:formgen-auth"`)
}

func defaultVanillaRegistry(t *testing.T) *render.Registry {
	t.Helper()

	registry := render.NewRegistry()
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("vanilla renderer: %v", err)
	}
	registry.MustRegister(renderer)
	return registry
}

func assertContains(t *testing.T, body, substr string) {
	t.Helper()
	if !strings.Contains(body, substr) {
		t.Fatalf("expected substring %q in output:\n%s", substr, body)
	}
}

func TestEndpointMetadata_SchemaMatchesOverride(t *testing.T) {
	overrideMetadata := gatherFieldMetadata(t, "relationships.yaml", orchestrator.WithEndpointOverrides([]orchestrator.EndpointOverride{
		{
			OperationID: "createArticle",
			FieldPath:   "article.author_id",
			Endpoint: orchestrator.EndpointConfig{
				URL:        "/api/authors",
				Method:     "GET",
				LabelField: "full_name",
				ValueField: "id",
				DynamicParams: map[string]string{
					"tenant_id": "{{field:tenant_id}}",
				},
				Auth: &orchestrator.EndpointAuth{
					Strategy: "header",
					Header:   "X-Auth-Token",
					Source:   "meta:formgen-auth",
				},
			},
		},
	}))

	schemaMetadata := gatherFieldMetadata(t, "relationships_with_endpoint.yaml")

	if diff := cmp.Diff(overrideMetadata, schemaMetadata); diff != "" {
		t.Fatalf("metadata mismatch (-override +schema):\n%s", diff)
	}
}

func gatherFieldMetadata(t *testing.T, fixture string, opts ...orchestrator.Option) map[string]string {
	t.Helper()

	ctx := testsupport.Context()
	source := pkgopenapi.SourceFromFile(filepath.Join("testdata", fixture))

	registry := render.NewRegistry()
	registry.MustRegister(&captureRenderer{})

	base := []orchestrator.Option{
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer("capture"),
		orchestrator.WithUISchemaFS(nil),
	}
	base = append(base, opts...)

	orch := orchestrator.New(base...)

	output, err := orch.Generate(ctx, orchestrator.Request{
		Source:      source,
		OperationID: "createArticle",
	})
	if err != nil {
		t.Fatalf("generate for %s: %v", fixture, err)
	}

	var form pkgmodel.FormModel
	if err := json.Unmarshal(output, &form); err != nil {
		t.Fatalf("unmarshal form: %v", err)
	}

	field := findField(form.Fields, []string{"article", "author_id"})
	if field == nil {
		t.Fatalf("author_id field missing in %s", fixture)
	}

	metadata := make(map[string]string, len(field.Metadata))
	for key, value := range field.Metadata {
		metadata[key] = value
	}
	return metadata
}

type captureRenderer struct{}

func (r *captureRenderer) Name() string {
	return "capture"
}

func (r *captureRenderer) ContentType() string {
	return "application/json"
}

func (r *captureRenderer) Render(_ context.Context, form pkgmodel.FormModel) ([]byte, error) {
	return json.Marshal(form)
}

func findField(fields []pkgmodel.Field, segments []string) *pkgmodel.Field {
	if len(segments) == 0 {
		return nil
	}
	for i := range fields {
		field := &fields[i]
		if field.Name != segments[0] {
			continue
		}
		if len(segments) == 1 {
			return field
		}
		if nested := findField(field.Nested, segments[1:]); nested != nil {
			return nested
		}
		if nested := findFieldInItem(field.Items, segments[1:]); nested != nil {
			return nested
		}
	}
	return nil
}

func findFieldInItem(item *pkgmodel.Field, segments []string) *pkgmodel.Field {
	if item == nil {
		return nil
	}
	if len(segments) == 0 {
		return item
	}
	if item.Name == segments[0] {
		if len(segments) == 1 {
			return item
		}
		if nested := findField(item.Nested, segments[1:]); nested != nil {
			return nested
		}
		return findFieldInItem(item.Items, segments[1:])
	}
	if nested := findField(item.Nested, segments); nested != nil {
		return nested
	}
	return findFieldInItem(item.Items, segments)
}
