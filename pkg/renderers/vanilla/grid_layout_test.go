package vanilla

import (
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestGridWrapperAttributes(t *testing.T) {
	tests := []struct {
		name    string
		hints   map[string]string
		columns int
		want    string
	}{
		{
			name:    "defaults to the full column count",
			columns: 12,
			want:    ` data-formgen-span="12"`,
		},
		{
			name:    "span start and row",
			hints:   map[string]string{"layout.span": "6", "layout.start": "4", "layout.row": "2"},
			columns: 12,
			want:    ` data-formgen-span="6" data-formgen-start="4" data-formgen-row="2"`,
		},
		{
			name: "breakpoints follow the sm..2xl order regardless of hint order",
			hints: map[string]string{
				"layout.span":     "12",
				"layout.span.xl":  "6",
				"layout.start.xl": "7",
				"layout.span.lg":  "6",
				"layout.row.sm":   "1",
			},
			columns: 12,
			want:    ` data-formgen-span="12" data-formgen-row-sm="1" data-formgen-span-lg="6" data-formgen-span-xl="6" data-formgen-start-xl="7"`,
		},
		{
			name:    "invalid and non-positive values fall back",
			hints:   map[string]string{"layout.span": "abc", "layout.start": "0", "layout.row": "-3", "layout.span.md": "x"},
			columns: 6,
			want:    ` data-formgen-span="6"`,
		},
		{
			name:    "unsupported breakpoint suffixes are ignored",
			hints:   map[string]string{"layout.span.xs": "3"},
			columns: 12,
			want:    ` data-formgen-span="12"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gridWrapperAttributes(model.Field{Name: "field", UIHints: tt.hints}, tt.columns)
			if got != tt.want {
				t.Fatalf("attributes mismatch\n got: %q\nwant: %q", got, tt.want)
			}
			if strings.Contains(got, "style=") {
				t.Fatalf("grid wrapper must not emit inline styles: %q", got)
			}
		})
	}
}

func TestNormalizeGutter(t *testing.T) {
	for raw, want := range map[string]string{
		"":      "md",
		"sm":    "sm",
		" LG ":  "lg",
		"md":    "md",
		"xl":    "md",
		"large": "md",
	} {
		if got := normalizeGutter(raw); got != want {
			t.Fatalf("normalizeGutter(%q) = %q, want %q", raw, got, want)
		}
	}
}
