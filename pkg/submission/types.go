// Package submission parses and validates submitted form values against the
// public form model contract.
package submission

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

// Values is the canonical submitted-value shape returned by parsers.
type Values map[string]any

// UnknownFieldPolicy controls how parser entry points handle fields that are
// not present in the supplied FormModel.
type UnknownFieldPolicy int

const (
	// UnknownIssue records an unknownField issue and drops the unknown value.
	UnknownIssue UnknownFieldPolicy = iota
	// UnknownIgnore silently drops unknown values.
	UnknownIgnore
	// UnknownPreserve preserves unknown values without recording an issue.
	UnknownPreserve
)

// EmptyStringPolicy controls how empty form strings are normalized.
type EmptyStringPolicy int

const (
	// EmptyDefault converts optional non-string scalar empty strings to nil and
	// keeps optional strings as "".
	EmptyDefault EmptyStringPolicy = iota
	// EmptyPreserve keeps empty strings exactly as submitted.
	EmptyPreserve
)

// EnumValueMode identifies whether strings are native control tokens or typed
// values. An encoded token can also be a legitimate literal in the same enum;
// its source, rather than its spelling, determines how it must be interpreted.
type EnumValueMode int

const (
	// EnumAuto preserves legacy generic-map behavior: declared strings are
	// literals, otherwise valid tokens decode. ParseValues resolves auto to native.
	EnumAuto EnumValueMode = iota
	// EnumControl decodes one transport layer, even when the token is declared.
	EnumControl
	// EnumLiteral preserves strings from an already typed payload.
	EnumLiteral
	// EnumNative uses field transport provenance: choices decode once, text
	// stays literal, and structural containers delegate to their children.
	EnumNative
)

// Options customize parsing and validation behavior.
type Options struct {
	UnknownFields UnknownFieldPolicy
	EmptyStrings  EmptyStringPolicy
	EnumValues    EnumValueMode
	MaxMemory     int64
	MaxBodyBytes  int64
	required      map[string]bool
	inactive      []string
}

// Option mutates Options.
type Option func(*Options)

const defaultMultipartMaxMemory = int64(32 << 20)
const defaultMultipartMaxBodyBytes = int64(64 << 20)

func defaultOptions() Options {
	return Options{
		UnknownFields: UnknownIssue,
		EmptyStrings:  EmptyDefault,
		MaxMemory:     defaultMultipartMaxMemory,
		MaxBodyBytes:  defaultMultipartMaxBodyBytes,
	}
}

func applyOptions(options []Option) Options {
	cfg := defaultOptions()
	for _, opt := range options {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.MaxMemory <= 0 {
		cfg.MaxMemory = defaultMultipartMaxMemory
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMultipartMaxBodyBytes
	}
	return cfg
}

// WithUnknownFields sets the parser unknown-field policy.
func WithUnknownFields(policy UnknownFieldPolicy) Option {
	return func(opts *Options) {
		opts.UnknownFields = policy
	}
}

// WithEmptyStrings sets empty-string normalization behavior.
func WithEmptyStrings(policy EmptyStringPolicy) Option {
	return func(opts *Options) {
		opts.EmptyStrings = policy
	}
}

// WithEnumValues selects the source of enum values. Use EnumLiteral for typed
// JSON and EnumControl for encoded control payloads. ParseMap/ParseJSON retain
// EnumAuto by default; ParseValues defaults to field-aware EnumNative.
// In native mode, custom controls can declare metadata submission.enumValues
// as "control" or "literal". Explicit global modes override field inference.
func WithEnumValues(mode EnumValueMode) Option {
	return func(opts *Options) { opts.EnumValues = mode }
}

// WithInactivePaths excludes trusted structural subtrees from validation and
// conditional projection without compacting array coordinates. Derive paths
// from validated host row intents; this option is not an authorization check.
func WithInactivePaths(paths ...string) Option {
	copy := append([]string(nil), paths...)
	return func(opts *Options) { opts.inactive = append([]string(nil), copy...) }
}

// WithMultipartMaxMemory sets the max memory used when parsing multipart
// requests.
func WithMultipartMaxMemory(maxMemory int64) Option {
	return func(opts *Options) {
		opts.MaxMemory = maxMemory
	}
}

// WithMultipartMaxBodyBytes sets the total body-size cap used before parsing
// multipart requests.
func WithMultipartMaxBodyBytes(maxBodyBytes int64) Option {
	return func(opts *Options) {
		opts.MaxBodyBytes = maxBodyBytes
	}
}

// Result contains parsed values and any non-fatal parse issues.
type Result struct {
	Values Values
	Issues []Issue
}

// Valid reports whether the result contains no issues.
func (r Result) Valid() bool {
	return len(r.Issues) == 0
}

// IssueCode is a stable machine-readable submission issue identifier.
type IssueCode string

const (
	CodeUnknownField  IssueCode = "unknownField"
	CodePathConflict  IssueCode = "pathConflict"
	CodeInvalidJSON   IssueCode = "invalidJSON"
	CodeRequired      IssueCode = "required"
	CodeType          IssueCode = "type"
	CodeEnum          IssueCode = "enum"
	CodeMin           IssueCode = "min"
	CodeMax           IssueCode = "max"
	CodeMinLength     IssueCode = "minLength"
	CodeMaxLength     IssueCode = "maxLength"
	CodePattern       IssueCode = "pattern"
	CodeMinItems      IssueCode = "minItems"
	CodeMaxItems      IssueCode = "maxItems"
	CodeObject        IssueCode = "object"
	CodeInvalidPath   IssueCode = "invalidPath"
	CodeUniqueItems   IssueCode = "uniqueItems"
	CodeInactiveInput IssueCode = "inactiveInput"
	CodeCondition     IssueCode = "condition"
)

// Issue describes a parser or validation problem.
type Issue struct {
	Code         IssueCode `json:"code"`
	Path         string    `json:"path,omitempty"`
	RendererPath string    `json:"rendererPath,omitempty"`
	Message      string    `json:"message"`
	Value        any       `json:"value,omitempty"`
}

func issue(code IssueCode, path, message string, value any) Issue {
	rendererPath := RendererPath(path)
	return Issue{
		Code:         code,
		Path:         path,
		RendererPath: rendererPath,
		Message:      message,
		Value:        value,
	}
}

func fieldLabel(field model.Field, fallback string) string {
	if label := strings.TrimSpace(field.Label); label != "" {
		return label
	}
	if name := strings.TrimSpace(field.Name); name != "" {
		return name
	}
	return fallback
}

func makeMessage(field model.Field, fallback, detail string) string {
	label := fieldLabel(field, fallback)
	if detail == "" {
		return label
	}
	return fmt.Sprintf("%s %s", label, detail)
}
