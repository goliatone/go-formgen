package orchestrator

import (
	"testing"

	pkgmodel "github.com/goliatone/go-formgen/pkg/model"
)

func TestApplyEndpointOverridesHandlesItemsPath(t *testing.T) {
	city := pkgmodel.Field{Name: "city", Type: pkgmodel.FieldTypeString}
	addresses := pkgmodel.Field{
		Name: "addresses",
		Type: pkgmodel.FieldTypeArray,
		Items: &pkgmodel.Field{
			Name:   "address",
			Type:   pkgmodel.FieldTypeObject,
			Nested: []pkgmodel.Field{city},
		},
	}

	form := pkgmodel.FormModel{Fields: []pkgmodel.Field{addresses}}

	o := &Orchestrator{
		endpointOverrides: map[string][]EndpointOverride{
			"createAddress": {
				{
					OperationID: "createAddress",
					FieldPath:   "addresses.items.city",
					Endpoint:    EndpointConfig{URL: "/api/cities"},
				},
			},
		},
	}

	o.applyEndpointOverrides("createAddress", &form)

	if form.Fields[0].Items == nil || len(form.Fields[0].Items.Nested) == 0 {
		t.Fatalf("expected nested item fields to remain")
	}
	patched := form.Fields[0].Items.Nested[0]
	if patched.Metadata == nil {
		t.Fatalf("metadata was not initialised on nested item")
	}
	if got := patched.Metadata["relationship.endpoint.url"]; got != "/api/cities" {
		t.Fatalf("relationship.endpoint.url = %q, want %q", got, "/api/cities")
	}
}
