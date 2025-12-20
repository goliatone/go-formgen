package timezones

import (
	"strings"
	"testing"
)

func TestLoadZones_DedupesSortsAndIgnoresComments(t *testing.T) {
	input := strings.NewReader(`
# Comment
America/New_York
Europe/Paris
America/New_York

UTC
`)

	zones, err := LoadZones(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(zones) != 3 {
		t.Fatalf("expected 3 zones, got %d", len(zones))
	}
	if zones[0] != "America/New_York" || zones[1] != "Europe/Paris" || zones[2] != "UTC" {
		t.Fatalf("unexpected zones: %#v", zones)
	}
}

func TestDefaultZones_ContainsCommonEntries(t *testing.T) {
	zones, err := DefaultZones()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(zones) < 200 {
		t.Fatalf("expected a reasonably sized list, got %d", len(zones))
	}

	for _, expected := range []string{"America/New_York", "Europe/Paris", "UTC"} {
		if !containsString(zones, expected) {
			t.Fatalf("expected zone %q to be present", expected)
		}
	}
}

func TestSearch_CaseInsensitiveContains(t *testing.T) {
	zones := []string{"Europe/Paris", "America/New_York", "UTC"}
	opts := NewOptions(WithEmptySearchMode(EmptySearchNone))

	results := Search(zones, "eUrOpE/p", 10, opts)
	if len(results) != 1 || results[0] != "Europe/Paris" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestSearch_PrefixBeforeContains(t *testing.T) {
	zones := []string{"x/a/b", "a/b", "a/b/c", "c/d"}
	opts := NewOptions(WithEmptySearchMode(EmptySearchNone))

	results := Search(zones, "a/b", 10, opts)
	want := []string{"a/b", "a/b/c", "x/a/b"}
	if len(results) != len(want) {
		t.Fatalf("expected %d results, got %d: %#v", len(want), len(results), results)
	}
	for i := range want {
		if results[i] != want[i] {
			t.Fatalf("unexpected ordering at %d: got %q want %q (results: %#v)", i, results[i], want[i], results)
		}
	}
}

func TestSearch_LimitApplied(t *testing.T) {
	zones := []string{"a", "b", "c", "d"}
	opts := NewOptions(WithDefaultLimit(2), WithMaxLimit(3), WithEmptySearchMode(EmptySearchTop))

	results := Search(zones, "", 0, opts)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %#v", len(results), results)
	}
}

func TestSearchOptions_MapsValueAndLabel(t *testing.T) {
	zones := []string{"UTC"}
	opts := NewOptions(WithEmptySearchMode(EmptySearchNone))

	results := SearchOptions(zones, "utc", 10, opts)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Value != "UTC" || results[0].Label != "UTC" {
		t.Fatalf("unexpected option: %#v", results[0])
	}
}

func containsString(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
