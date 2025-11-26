package render_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/goliatone/formgen/pkg/render"
)

func TestMergeAndSortHiddenFields(t *testing.T) {
	base := map[string]string{
		" existing ": "keep",
		"":           "ignored",
	}

	merged := render.MergeHiddenFields(base,
		render.CSRFToken("_csrf", "token123"),
		render.AuthToken(" auth_token ", "abc123"),
		render.VersionField("version", 4),
		render.Hidden("  ", "skip"),
	)

	wantMerged := map[string]string{
		"existing":   "keep",
		"_csrf":      "token123",
		"auth_token": "abc123",
		"version":    "4",
	}
	if diff := cmp.Diff(wantMerged, merged); diff != "" {
		t.Fatalf("merged hidden fields mismatch (-want +got):\n%s", diff)
	}

	sorted := render.SortedHiddenFields(merged)
	wantSorted := []render.HiddenField{
		{Name: "_csrf", Value: "token123"},
		{Name: "auth_token", Value: "abc123"},
		{Name: "existing", Value: "keep"},
		{Name: "version", Value: "4"},
	}
	if diff := cmp.Diff(wantSorted, sorted); diff != "" {
		t.Fatalf("sorted hidden fields mismatch (-want +got):\n%s", diff)
	}
}
