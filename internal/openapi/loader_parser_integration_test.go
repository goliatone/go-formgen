package openapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/goliatone/formgen"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

func TestLoaderParserIntegration(t *testing.T) {
	t.Skip("pending implementation")

	ctx := context.Background()

	fixture := filepath.Join("testdata", "petstore.yaml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "petstore.yaml")
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}

	loader := formgen.NewLoader()

	// File source
	docFile, err := loader.Load(ctx, pkgopenapi.SourceFromFile(filePath))
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	parser := formgen.NewParser()
	if _, err := parser.Operations(ctx, docFile); err != nil {
		t.Fatalf("parse file document: %v", err)
	}

	// HTTP source
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	loaderHTTP := formgen.NewLoader(pkgopenapi.WithHTTPFallback(0))
	docHTTP, err := loaderHTTP.Load(ctx, pkgopenapi.SourceFromURL(server.URL))
	if err != nil {
		t.Fatalf("load http: %v", err)
	}
	if _, err := parser.Operations(ctx, docHTTP); err != nil {
		t.Fatalf("parse http document: %v", err)
	}
}
