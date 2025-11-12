package main

import "github.com/goliatone/formgen/pkg/render"

func sampleRenderOptionsFor(key string) (render.RenderOptions, bool) {
	switch key {
	case "article":
		return render.RenderOptions{
			Method: "PATCH",
			Values: map[string]any{
				"title":               "Existing article title",
				"slug":                "existing-article-title",
				"summary":             "Updated teaser copy for the story.",
				"tenant_id":           "garden",
				"status":              "scheduled",
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
		}, true
	default:
		return render.RenderOptions{}, false
	}
}
