package uischema

import (
	"strings"
	"testing"
)

func TestSanitizeIconMarkupRemovesScripts(t *testing.T) {
	input := `  <svg viewBox="0 0 24 24"><script>alert('x')</script><path d="M0 0h24v24H0z" /></svg>`
	got := sanitizeIconMarkup(input)
	if got == "" {
		t.Fatalf("expected sanitized markup, got empty string")
	}
	if containsScript := contains(got, "script"); containsScript {
		t.Fatalf("expected script tag to be removed, got %q", got)
	}
	if !contains(got, "<svg") || !contains(got, "<path") {
		t.Fatalf("expected svg/path elements to remain, got %q", got)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
