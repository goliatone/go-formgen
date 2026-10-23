package model

import (
	"encoding/json"
	"fmt"
	"github.com/goliatone/go-formgen/pkg/schema"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestPortableDefinitionFixtures(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/portable-definitions.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Patterns []struct {
			Pattern    string
			Accepted   bool
			Pass, Fail []string
		}
		Definitions []struct {
			Name, Source string
			Accepted     bool
		}
	}
	if err = json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.Patterns {
		t.Run(c.Pattern, func(t *testing.T) {
			if got := IsPortablePattern(c.Pattern); got != c.Accepted {
				t.Fatalf("accept=%v want %v", got, c.Accepted)
			}
			if !c.Accepted {
				return
			}
			r := regexp.MustCompile(c.Pattern)
			for _, s := range c.Pass {
				if !r.MatchString(s) {
					t.Fatalf("expected match %q", s)
				}
			}
			for _, s := range c.Fail {
				if r.MatchString(s) {
					t.Fatalf("unexpected match %q", s)
				}
			}
		})
	}
	for _, c := range f.Definitions {
		t.Run(c.Name, func(t *testing.T) {
			d, err := DiagnosePortableDefinitionJSON([]byte(c.Source))
			if err != nil {
				t.Fatal(err)
			}
			if (len(d) == 0) != c.Accepted {
				t.Fatalf("diagnostics=%+v", d)
			}
		})
	}
}
func TestPortableDefinitionOriginalJSON(t *testing.T) {
	for _, source := range []string{`{"type":"integer","default":9007199254740991.1}`, `{"type":"integer","default":1e100}`, `{"type":"integer","enum":[1.1]}`, `{"pattern":"^A\\sB$"}`} {
		d, err := DiagnosePortableDefinitionJSON([]byte(source))
		if err != nil || len(d) == 0 {
			t.Fatalf("%s: %+v %v", source, d, err)
		}
	}
	if _, err := DiagnosePortableDefinitionJSON([]byte(`{} {}`)); err == nil {
		t.Fatal("multiple documents accepted")
	}
}

func TestPortableBuilderOptInPreservesHeadlessPatterns(t *testing.T) {
	form := schema.Form{ID: "portable", Endpoint: "/portable", Method: "POST", Schema: schema.Schema{Type: "object", Properties: map[string]schema.Schema{"answer": {Type: "string", Pattern: `^A\sB$`}}}}
	if _, err := NewBuilder().Build(form); err != nil {
		t.Fatalf("legacy headless: %v", err)
	}
	if _, err := NewBuilder(WithPortableDefinitions()).Build(form); err == nil {
		t.Fatal("portable builder accepted shorthand whitespace")
	}
	for _, raw := range []string{`{"type":"integer","default":1e999999999}`, `{"type":"integer","default":1e-999999999}`} {
		if d, err := DiagnosePortableDefinitionJSON([]byte(raw)); err != nil || len(d) == 0 {
			t.Fatalf("large exponent not rejected: %v %v", d, err)
		}
	}
}

func TestPortableDefinitionNamedSchemaMaps(t *testing.T) {
	for _, name := range []string{"metadata", "uiHints", "x-special", "default", "enum", "const", "options", "conditions", "a/b~c"} {
		for _, wrapper := range []string{
			`{"type":"object","properties":{%s:%s}}`,
			`{"$defs":{%s:%s}}`,
			`{"openapi":"3.1.0","components":{"schemas":{%s:%s}}}`,
			`{"openapi":"3.1.0","paths":{"/test":{"get":{"responses":{"200":{"content":{"application/json":{"schema":{"properties":{%s:%s}}}}}}}}}}`,
		} {
			key, _ := json.Marshal(name)
			source := fmt.Sprintf(wrapper, key, `{"type":"integer","default":9007199254740993,"pattern":"^A\\sB$"}`)
			diagnostics, err := DiagnosePortableDefinitionJSON([]byte(source))
			if err != nil || len(diagnostics) != 2 {
				t.Fatalf("%s: %+v %v", source, diagnostics, err)
			}
			escaped := strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
			for _, d := range diagnostics {
				if !strings.Contains(d.Path, "/"+escaped+"/") {
					t.Fatalf("incorrect path: %+v", d)
				}
			}
		}
	}
	// Opaque host namespaces and literal answer objects are not schema nodes.
	source := `{"type":"object","metadata":{"pattern":".*","default":9007199254740993},"x-host":{"pattern":".*"},"default":{"type":"integer","pattern":".*","default":4},"enum":[{"pattern":".*"}],"properties":{"answer":{"type":"string","pattern":"^[A-Z]+$"}}}`
	if d, err := DiagnosePortableDefinitionJSON([]byte(source)); err != nil || len(d) != 0 {
		t.Fatalf("opaque content diagnosed: %+v %v", d, err)
	}
}
