package uischema

import (
	"strings"
	"testing"
)

func TestSanitizeIconMarkupAllowsSafeSVG(t *testing.T) {
	input := `  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="6"/><path d="M16 16L21 21"/></svg>  `

	got := sanitizeIconMarkup(input)

	if got == "" {
		t.Fatalf("expected sanitized markup, got empty string")
	}
	assertContains(t, got, "<svg")
	assertContains(t, got, "<circle")
	assertContains(t, got, "<path")
	assertContains(t, got, `stroke="currentColor"`)
	assertContains(t, got, `stroke-width="1.5"`)
	assertNotContains(t, got, "script")
}

func TestSanitizeIconMarkupRemovesUnsafeElements(t *testing.T) {
	input := `<svg viewBox="0 0 24 24"><defs><linearGradient id="g"><stop offset="0" stop-color="red"/></linearGradient></defs><foreignObject><iframe src="https://example.com"></iframe></foreignObject><script>alert("x")</script><path d="M0 0h24v24H0z"/></svg>`

	got := sanitizeIconMarkup(input)

	if got == "" {
		t.Fatalf("expected sanitized markup, got empty string")
	}
	assertContains(t, got, "<linearGradient")
	assertContains(t, got, "<stop")
	assertContains(t, got, "<path")
	assertNotContains(t, got, "foreignObject")
	assertNotContains(t, got, "iframe")
	assertNotContains(t, got, "script")
}

func TestSanitizeIconMarkupRemovesUnsafeAttributes(t *testing.T) {
	input := `<svg viewBox="0 0 24 24" onload="alert(1)" style="background:url(javascript:alert(1))"><a href="javascript:alert(1)" xlink:href="data:image/svg+xml;base64,PHN2Zy8+"><path d="M0 0h24v24H0z" onclick="alert(2)" data-name="bad" fill="currentColor"/></a></svg>`

	got := sanitizeIconMarkup(input)

	if got == "" {
		t.Fatalf("expected sanitized markup, got empty string")
	}
	assertContains(t, got, "<a>")
	assertContains(t, got, `fill="currentColor"`)
	assertNotContains(t, got, "onload")
	assertNotContains(t, got, "onclick")
	assertNotContains(t, got, "style=")
	assertNotContains(t, got, "href=")
	assertNotContains(t, got, "data-name")
	assertNotContains(t, got, "javascript:")
	assertNotContains(t, got, "data:image")
}

func TestSanitizeIconMarkupKeepsSafeFragmentLinks(t *testing.T) {
	input := `<svg viewBox="0 0 24 24"><defs><path id="shape" d="M0 0h1v1H0z"/></defs><use href="#shape" xlink:href="#shape" width="24" height="24"/></svg>`

	got := sanitizeIconMarkup(input)

	assertContains(t, got, `href="#shape"`)
	assertContains(t, got, `xlink:href="#shape"`)
	assertContains(t, got, "<use")
}

func TestSanitizeIconMarkupRejectsNonSVGRoot(t *testing.T) {
	input := `<div><svg viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg></div>`

	if got := sanitizeIconMarkup(input); got != "" {
		t.Fatalf("expected non-SVG root to be rejected, got %q", got)
	}
}

func TestSanitizeIconMarkupRejectsMalformedSVG(t *testing.T) {
	input := `<svg viewBox="0 0 24 24"><path d="M0 0h24v24H0z"></svg`

	if got := sanitizeIconMarkup(input); got != "" {
		t.Fatalf("expected malformed SVG to be rejected, got %q", got)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected %q not to contain %q", haystack, needle)
	}
}
