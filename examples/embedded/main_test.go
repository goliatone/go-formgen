package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
)

func TestValidateEndpointPreservesTypedLiteralStrings(t *testing.T) {
	server := &exampleServer{form: model.FormModel{Fields: []model.Field{{
		Name: "answer", Type: model.FieldTypeString, Required: true,
		Validations: []model.ValidationRule{{Kind: model.ValidationRulePattern, Params: map[string]string{"pattern": "^__fg_enum_v1:"}}},
	}}}}
	body, err := json.Marshal(map[string]any{"answer": submission.EncodeEnumValue(false)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleValidate(recorder, httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(string(body))))
	var response validateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || recorder.Code != http.StatusOK || len(response.Issues) != 0 {
		t.Fatalf("typed literal was decoded as transport: status=%d body=%s err=%v", recorder.Code, recorder.Body.String(), err)
	}
}

func TestExamplePageRendersFormDescriptorAndExtension(t *testing.T) {
	server, err := newExampleServer(context.Background())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	page := string(server.page)
	for _, marker := range []string{
		"<form",
		`id="formgen-descriptor"`,
		`data-component="note-counter"`,
		`name="colors`,
		`/runtime/formgen-relationships.min.js`,
		`/adapter.js`,
	} {
		if !strings.Contains(page, marker) {
			t.Fatalf("page missing %q", marker)
		}
	}
	if server.form.Conditions == nil || len(server.form.Conditions.Rules) != 1 {
		t.Fatalf("authoritative conditions missing: %+v", server.form.Conditions)
	}
	var descriptor struct {
		Version string `json:"version"`
		Form    struct {
			Fields     []struct{ Name string }   `json:"fields"`
			Conditions *struct{ Version string } `json:"conditions"`
		} `json:"form"`
	}
	if err := json.Unmarshal(server.descriptor, &descriptor); err != nil {
		t.Fatalf("descriptor JSON: %v", err)
	}
	if descriptor.Version != "formgen.descriptor/v1" || descriptor.Form.Conditions == nil {
		t.Fatalf("descriptor lacks version or conditions: %+v", descriptor)
	}
}

func TestPageWorksUnderStrictPolicy(t *testing.T) {
	server, err := newExampleServer(context.Background())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	page := string(server.page)
	if strings.Contains(page, "<style") {
		t.Fatalf("page must not inline styles:\n%s", page)
	}
	// The vanilla grid layout travels as data-formgen-* attributes; an inline
	// style attribute anywhere would be blocked by style-src 'self'.
	if strings.Contains(page, ` style="`) {
		t.Fatalf("page must not carry inline style attributes:\n%s", page)
	}
	if !strings.Contains(strictCSP, "style-src 'self';") || strings.Contains(strictCSP, "unsafe-inline") {
		t.Fatalf("strict policy must restrict styles to 'self': %s", strictCSP)
	}
	// Every script element is either external (src=) or a non-executable JSON data block.
	for _, segment := range strings.Split(page, "<script")[1:] {
		if !strings.HasPrefix(segment, " src=") && !strings.HasPrefix(segment, ` type="application/json"`) {
			t.Fatalf("inline script found: %s", segment[:min(len(segment), 80)])
		}
	}
	for _, marker := range []string{`href="/example.css"`, `href="/vanilla/formgen-vanilla.css"`, `data-embedded-action="reference"`} {
		if !strings.Contains(page, marker) {
			t.Fatalf("page missing %q", marker)
		}
	}
	server.csp = true
	handler := server.routes()
	for _, target := range []string{"/", "/example.css", "/vanilla/formgen-vanilla.css", "/runtime/formgen-relationships.min.js", "/adapter.js"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status %d", target, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Security-Policy"); got != strictCSP {
			t.Fatalf("%s policy %q", target, got)
		}
	}
}

func TestValidateEndpoint(t *testing.T) {
	server, err := newExampleServer(context.Background())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	handler := server.routes()

	post := func(body string) (int, []string, string) {
		request := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			return recorder.Code, nil, recorder.Body.String()
		}
		var payload struct {
			Issues []struct {
				Code string `json:"code"`
				Path string `json:"path"`
			} `json:"issues"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		codes := make([]string, 0, len(payload.Issues))
		for _, issue := range payload.Issues {
			codes = append(codes, issue.Code+":"+issue.Path)
		}
		// Normalized schema fields are ordered by name; compare as sets.
		sort.Strings(codes)
		return recorder.Code, codes, recorder.Body.String()
	}

	cases := []struct {
		name string
		body string
		want []string
	}{
		{name: "C01 missing", body: `{}`, want: []string{"required:enabled", "required:colors"}},
		{name: "C02 false passes", body: `{"enabled":false,"colors":["blue"]}`, want: []string{}},
		{name: "C03 conditional required", body: `{"enabled":true,"colors":["blue"]}`, want: []string{"required:level"}},
		{name: "C05 range", body: `{"enabled":true,"level":11,"colors":["blue"]}`, want: []string{"max:level"}},
		{name: "C06 enum", body: `{"enabled":true,"level":4,"colors":["red"]}`, want: []string{"enum:colors[0]"}},
		{name: "C07 inactive input rejected", body: `{"enabled":false,"level":4,"colors":["blue"]}`, want: []string{"inactiveInput:level"}},
		{name: "valid", body: `{"enabled":true,"level":4,"colors":["blue"],"note":"ok","contact":{"email":"a@example.com"}}`, want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, codes, raw := post(tc.body)
			if status != http.StatusOK {
				t.Fatalf("status %d: %s", status, raw)
			}
			sort.Strings(tc.want)
			if strings.Join(codes, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("issues %v, want %v (%s)", codes, tc.want, raw)
			}
			if strings.Contains(raw, `"value"`) {
				t.Fatalf("response leaked submitted values: %s", raw)
			}
		})
	}

	// Malformed or non-object bodies answer 400 with only the parse issue.
	for _, body := range []string{`{"enabled":`, `[1,2]`} {
		request := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %q status %d: %s", body, recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), `"path":"enabled"`) {
			t.Fatalf("body %q validated fields after a parse failure: %s", body, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/validate", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown route status %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/runtime/formgen-relationships.min.js", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("runtime bundle status %d", recorder.Code)
	}
}
