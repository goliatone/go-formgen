package formgenwiring

import (
	"testing"

	"github.com/goliatone/go-formgen/components/timezones"
)

func TestTimezonesEndpointOverride_Defaults(t *testing.T) {
	ov := TimezonesEndpointOverride("op", "timezone", "/admin")

	if ov.OperationID != "op" {
		t.Fatalf("unexpected operation id: %q", ov.OperationID)
	}
	if ov.FieldPath != "timezone" {
		t.Fatalf("unexpected field path: %q", ov.FieldPath)
	}
	if ov.Endpoint.URL != "/admin/api/timezones" {
		t.Fatalf("unexpected url: %q", ov.Endpoint.URL)
	}
	if ov.Endpoint.Method != "GET" {
		t.Fatalf("unexpected method: %q", ov.Endpoint.Method)
	}
	if ov.Endpoint.ResultsPath != "data" {
		t.Fatalf("unexpected results path: %q", ov.Endpoint.ResultsPath)
	}
	if got := ov.Endpoint.Params["format"]; got != "options" {
		t.Fatalf("unexpected format param: %q", got)
	}
	if got := ov.Endpoint.Params["limit"]; got != "50" {
		t.Fatalf("unexpected limit param: %q", got)
	}
	if got := ov.Endpoint.DynamicParams["q"]; got != "{{self}}" {
		t.Fatalf("unexpected dynamic q param: %q", got)
	}
	if ov.Endpoint.Mapping.Value != "value" || ov.Endpoint.Mapping.Label != "label" {
		t.Fatalf("unexpected mapping: %#v", ov.Endpoint.Mapping)
	}
}

func TestTimezonesEndpointOverride_CustomParams(t *testing.T) {
	ov := TimezonesEndpointOverride(
		"op",
		"timezone",
		"/admin",
		timezones.WithRoutePath("/api/tz"),
		timezones.WithSearchParam("search"),
		timezones.WithLimitParam("l"),
		timezones.WithDefaultLimit(10),
	)

	if ov.Endpoint.URL != "/admin/api/tz" {
		t.Fatalf("unexpected url: %q", ov.Endpoint.URL)
	}
	if got := ov.Endpoint.Params["l"]; got != "10" {
		t.Fatalf("unexpected limit param: %q", got)
	}
	if got := ov.Endpoint.DynamicParams["search"]; got != "{{self}}" {
		t.Fatalf("unexpected dynamic search param: %q", got)
	}
	if _, ok := ov.Endpoint.DynamicParams["q"]; ok {
		t.Fatalf("did not expect default dynamic param to remain present: %#v", ov.Endpoint.DynamicParams)
	}
}
