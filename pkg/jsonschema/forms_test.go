package jsonschema

import "testing"

func TestDiscoverFormsFromBytes_ExplicitForms(t *testing.T) {
	raw := []byte(`{
  "x-formgen": {
    "forms": [
      {"id": "post.edit", "title": "Edit Post"},
      {"id": "post.create"}
    ]
  }
}`)

	refs, err := DiscoverFormsFromBytes(raw, FormDiscoveryOptions{})
	if err != nil {
		t.Fatalf("discover forms: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 forms, got %d", len(refs))
	}
	if refs[0].ID != "post.edit" || refs[0].Title != "Edit Post" {
		t.Fatalf("unexpected first form: %+v", refs[0])
	}
	if refs[1].ID != "post.create" {
		t.Fatalf("unexpected second form: %+v", refs[1])
	}
}

func TestDiscoverFormsFromBytes_ExplicitFormsMissingID(t *testing.T) {
	raw := []byte(`{"x-formgen":{"forms":[{"title":"Missing ID"}]}}`)

	_, err := DiscoverFormsFromBytes(raw, FormDiscoveryOptions{})
	if err == nil {
		t.Fatalf("expected error for missing id")
	}
}

func TestDiscoverFormsFromBytes_FromID(t *testing.T) {
	raw := []byte(`{"$id":"com.example.post"}`)

	refs, err := DiscoverFormsFromBytes(raw, FormDiscoveryOptions{})
	if err != nil {
		t.Fatalf("discover forms: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 form, got %d", len(refs))
	}
	if refs[0].ID != "com.example.post.edit" {
		t.Fatalf("unexpected form id: %s", refs[0].ID)
	}
}

func TestDiscoverFormsFromBytes_FromSlug(t *testing.T) {
	raw := []byte(`{}`)

	refs, err := DiscoverFormsFromBytes(raw, FormDiscoveryOptions{Slug: "article"})
	if err != nil {
		t.Fatalf("discover forms: %v", err)
	}
	if len(refs) != 1 || refs[0].ID != "article.edit" {
		t.Fatalf("unexpected form refs: %+v", refs)
	}
}

func TestDiscoverFormsFromBytes_CustomSuffix(t *testing.T) {
	raw := []byte(`{}`)

	refs, err := DiscoverFormsFromBytes(raw, FormDiscoveryOptions{Slug: "article", FormIDSuffix: "update"})
	if err != nil {
		t.Fatalf("discover forms: %v", err)
	}
	if len(refs) != 1 || refs[0].ID != "article.update" {
		t.Fatalf("unexpected form refs: %+v", refs)
	}
}

func TestDiscoverFormsFromBytes_MissingSlug(t *testing.T) {
	raw := []byte(`{}`)

	_, err := DiscoverFormsFromBytes(raw, FormDiscoveryOptions{})
	if err == nil {
		t.Fatalf("expected error for missing slug")
	}
}
