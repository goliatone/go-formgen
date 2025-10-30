package template_test

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goliatone/formgen/pkg/render/template/gotemplate"
	"github.com/goliatone/formgen/pkg/testsupport"
)

//go:embed testdata/templates/*.tpl
var embeddedTemplates embed.FS

func TestGoTemplateEngine_RenderTemplate(t *testing.T) {
	engine := newEngine(t)

	result, written := testsupport.CaptureTemplateOutput(t, func(w io.Writer) (string, error) {
		return engine.RenderTemplate("hello", map[string]any{"name": "Ada"}, w)
	})

	want := testsupport.MustReadGoldenString(t, filepath.Join("testdata", "hello.golden"))
	if result != want {
		t.Fatalf("render template mismatch result\nwant: %q\n got: %q", want, result)
	}
	if written != want {
		t.Fatalf("render template mismatch writer\nwant: %q\n got: %q", want, written)
	}
}

func TestGoTemplateEngine_GlobalContext(t *testing.T) {
	engine := newEngine(t)
	if err := engine.GlobalContext(map[string]any{
		"settings": map[string]any{"env": "staging"},
	}); err != nil {
		t.Fatalf("global context: %v", err)
	}

	result, written := testsupport.CaptureTemplateOutput(t, func(w io.Writer) (string, error) {
		return engine.RenderTemplate("use-global", nil, w)
	})

	want := testsupport.MustReadGoldenString(t, filepath.Join("testdata", "use-global.golden"))
	if result != want {
		t.Fatalf("render template mismatch result\nwant: %q\n got: %q", want, result)
	}
	if written != want {
		t.Fatalf("render template mismatch writer\nwant: %q\n got: %q", want, written)
	}
}

func TestGoTemplateEngine_RegisterFilter(t *testing.T) {
	engine := newEngine(t)
	err := engine.RegisterFilter("shout", func(input any, _ any) (any, error) {
		if input == nil {
			return "", nil
		}
		return fmt.Sprintf("%s!", strings.ToUpper(fmt.Sprint(input))), nil
	})
	if err != nil {
		t.Fatalf("register filter: %v", err)
	}

	result, written := testsupport.CaptureTemplateOutput(t, func(w io.Writer) (string, error) {
		return engine.RenderTemplate("use-filter", map[string]any{"name": "Ada"}, w)
	})

	want := testsupport.MustReadGoldenString(t, filepath.Join("testdata", "use-filter.golden"))
	if result != want {
		t.Fatalf("render template mismatch result\nwant: %q\n got: %q", want, result)
	}
	if written != want {
		t.Fatalf("render template mismatch writer\nwant: %q\n got: %q", want, written)
	}
}

func newEngine(t *testing.T) *gotemplate.Engine {
	t.Helper()

	templatesFS, err := fs.Sub(embeddedTemplates, "testdata/templates")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}

	engine, err := gotemplate.New(gotemplate.WithFS(templatesFS))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine
}
