package main

import "github.com/goliatone/go-formgen/pkg/render"

var articleSample = render.RenderOptions{
	Method: "PATCH",
	Values: map[string]any{
		"title":             "Existing article title",
		"slug":              "existing-article-title",
		"summary":           "Updated teaser copy for the story.",
		"tenant_id":         "northwind", // Must match author/tag tenant for API lookup
		"status":            "scheduled",
		"hero_image":        "https://placehold.co/1200x800.png?text=Existing+Hero",
		"read_time_minutes": 7,
		// Relationship fields: use IDs that exist in the mock API dataset.
		// Labels are resolved from the API response, not stored here.
		"author_id":           "11111111-1111-4111-8111-111111111111", // Ada Lovelace
		"manager_id":          "88888888-8888-4888-8888-888888888888", // Grace Hopper
		"category_id":         "55555555-5555-4555-8555-555555555555", // Engineering
		"tags":                []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}, // feature, announcement
		"related_article_ids": []string{},
		"published_at":        "2024-03-01T10:00:00Z",
		"cta.headline":        "Ready to dig deeper?",
		"cta.url":             "https://example.com/cta",
		"cta.button_text":     "Explore guides",
		"seo.title":           "Existing article title | Northwind Editorial",
		"seo.description":     "Updated description for SEO block.",
	},
	Errors: map[string][]string{
		"slug":       {"Slug already taken"},
		"manager_id": {"Manager must belong to the selected author"},
	},
}

var bookSample = render.RenderOptions{
	Values: map[string]any{
		"title":        "The Art of Code",
		"isbn":         "978-0-00-000001-0",
		"status":       "in_review",
		"author_id":    "11111111-1111-4111-8111-111111111111", // Ada Lovelace
		"publisher_id": "cccc3333-cccc-4333-8333-cccccccccccc", // Northwind Publishing
		"tags":         []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "cccccccc-cccc-4ccc-8ccc-cccccccccccc"},
		// JSON editor field: JSON string value for proper prefill
		// Note: The JSON editor component expects a JSON string for the textarea.
		// Native Go maps would be flattened by the prefill logic.
		"metadata": `{
  "awards": ["Hugo Award 2023", "Nebula Nominee"],
  "editions": 3,
  "translated": true,
  "languages": ["en", "es", "fr"],
  "formats": {
    "hardcover": true,
    "paperback": true,
    "ebook": true,
    "audiobook": false
  }
}`,
	},
}

var presetRenderOptions = map[string]render.RenderOptions{
	"article":      articleSample,
	"article-edit": articleSample,
	"book":         bookSample,
	"book-create":  bookSample,
}

func sampleRenderOptionsFor(key string) (render.RenderOptions, bool) {
	sample, ok := presetRenderOptions[key]
	if !ok {
		return render.RenderOptions{}, false
	}
	return cloneRenderOptions(sample), true
}

func cloneRenderOptions(sample render.RenderOptions) render.RenderOptions {
	clone := render.RenderOptions{
		Method: sample.Method,
		Values: cloneAnyMap(sample.Values),
		Errors: cloneStringSliceMap(sample.Errors),
	}
	return clone
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneStringSliceMap(source map[string][]string) map[string][]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string][]string, len(source))
	for key, values := range source {
		if len(values) == 0 {
			result[key] = nil
			continue
		}
		dup := make([]string, len(values))
		copy(dup, values)
		result[key] = dup
	}
	return result
}
