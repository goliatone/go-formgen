package main

import "github.com/goliatone/go-formgen/pkg/render"

var articleSample = render.RenderOptions{
	Method: "PATCH",
	Values: map[string]any{
		"title":               "Existing article title",
		"slug":                "existing-article-title",
		"summary":             "Updated teaser copy for the story.",
		"tenant_id":           "garden",
		"status":              "scheduled",
		"hero_image":          "https://placehold.co/1200x800.png?text=Existing+Hero",
		"read_time_minutes":   7,
		"author_id":           "11111111-1111-4111-8111-111111111111",
		"manager_id":          "88888888-8888-4888-8888-888888888888",
		"category_id":         "55555555-5555-4555-8555-555555555555",
		"tags":                []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"},
		"related_article_ids": []string{"22222222-2222-4222-8222-222222222222"},
		"published_at":        "2024-03-01T10:00:00Z",
		"cta.headline":        "Ready to dig deeper?",
		"cta.url":             "https://example.com/cta",
		"cta.button_text":     "Explore guides",
		"seo.title":           "Existing article title | Northwind Editorial",
		"seo.description":     "Updated description for SEO block.",
	},
	Errors: map[string][]string{
		"slug":                {"Slug already taken"},
		"manager_id":          {"Manager must belong to the selected author"},
		"tags":                {"Select at least one tag", "Tags must match the tenant"},
		"title":               {"Title cannot be empty"},
		"related_article_ids": {"Replace duplicate related articles"},
	},
}

var presetRenderOptions = map[string]render.RenderOptions{
	"article":      articleSample,
	"article-edit": articleSample,
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
