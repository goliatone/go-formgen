package render

import (
	"errors"
	"strings"

	"github.com/goliatone/go-formgen/pkg/visibility"
	"github.com/goliatone/go-theme"
)

// Translator resolves a string for a given locale and message key.
// It matches github.com/goliatone/go-i18n's Translator interface so downstream
// projects can pass that implementation without introducing a hard dependency.
type Translator interface {
	Translate(locale, key string, args ...any) (string, error)
}

// MissingTranslationHandler decides what string should be emitted when a
// translation cannot be resolved.
//
// Convention: when go-formgen performs model localization it passes a single
// map argument in args containing { "default": <existing text> }.
type MissingTranslationHandler func(locale, key string, args []any, err error) string

var ErrMissingTranslator = errors.New("render: missing translator")

// RenderOptions describe per-request data that renderers can use to customise
// their output without mutating the form model pipeline.
type RenderOptions struct {
	// Method overrides the HTTP method declared by the form model. Renderers are
	// responsible for translating unsupported verbs (PATCH/PUT/DELETE) into
	// browser-friendly POST submissions plus a hidden _method input when needed.
	Method string
	// Subset restricts rendering to fields whose group, tags, or section match
	// the supplied tokens. Empty subsets leave the form unchanged.
	Subset FieldSubset
	// Values pre-populates rendered controls using dotted field paths (e.g.
	// "author.email"). Values may be wrapped in ValueWithProvenance to attach
	// provenance labels or lock fields as readonly/disabled. Renderers can
	// decide how to handle nested values or collections for advanced components
	// such as chips/typeahead controls.
	Values map[string]any
	// Errors surfaces server-side validation feedback keyed by field path. The
	// vanilla renderer maps these into inline chrome plus data-validation
	// attributes so the runtime and assistive tech can reflect the state without
	// waiting for client-side validation.
	Errors map[string][]string
	// FormErrors carries non-field-specific validation messages that should
	// render at the top of the form. These often come from request-scoped or
	// cross-field validation in the backend.
	FormErrors []string
	// HiddenFields injects name/value pairs as hidden inputs, useful for CSRF
	// tokens, auth/session hints, optimistic locking versions, or other
	// submission metadata that should travel with the form without showing up in
	// the visible schema.
	HiddenFields map[string]string
	// Locale selects the locale used by render-time localization helpers.
	Locale string
	// Translator enables model and template-level localization.
	Translator Translator
	// OnMissing customizes what string is used when a translation is missing.
	OnMissing MissingTranslationHandler
	// Theme passes renderer configuration derived from a go-theme Selection so
	// renderers can resolve partials, assets, and tokens consistently.
	Theme *theme.RendererConfig
	// VisibilityContext carries evaluator-specific inputs such as current form
	// values or feature flags used to decide whether a field should render.
	VisibilityContext visibility.Context
	// TopPadding controls how many leading newlines renderers emit before the
	// root form markup when no external stylesheets or inline styles are
	// present. A zero value allows the renderer to apply its default.
	TopPadding int
	// OmitAssets instructs renderers to skip emitting <link>, <style>, and
	// <script> tags. This is useful for rendering partial forms (e.g., modal
	// bodies) that will be embedded in a page where the parent already supplies
	// these assets.
	OmitAssets bool
}

func missingTranslationDefault(locale, key string, args []any, err error) string {
	def := ""
	if len(args) > 0 {
		if meta, ok := args[0].(map[string]any); ok {
			if v, ok := meta["default"]; ok {
				def = strings.TrimSpace(anyToString(v))
			}
		}
	}
	if def != "" {
		return def
	}
	return key
}

// FieldSubset describes the allowed groups, tags, or sections for partial
// rendering. When all slices are empty the form is left untouched.
type FieldSubset struct {
	Groups   []string
	Tags     []string
	Sections []string
}

// ValueWithProvenance attaches optional provenance metadata to a prefilled
// value. Renderers can surface the provenance label and enforce readonly or
// disabled state when provided.
type ValueWithProvenance struct {
	Value      any
	Provenance string
	Readonly   bool
	Disabled   bool
}

// PrefillValue is a convenience helper for constructing ValueWithProvenance
// entries when the caller only needs to specify the value and provenance label.
func PrefillValue(value any, provenance string) ValueWithProvenance {
	return ValueWithProvenance{
		Value:      value,
		Provenance: provenance,
	}
}
