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

// Options customize parsing and validation behavior.
type Options struct {
	UnknownFields UnknownFieldPolicy
	EmptyStrings  EmptyStringPolicy
	MaxMemory     int64
}

// Option mutates Options.
type Option func(*Options)

const defaultMultipartMaxMemory = int64(32 << 20)

func defaultOptions() Options {
	return Options{
		UnknownFields: UnknownIssue,
		EmptyStrings:  EmptyDefault,
		MaxMemory:     defaultMultipartMaxMemory,
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

// WithMultipartMaxMemory sets the max memory used when parsing multipart
// requests.
func WithMultipartMaxMemory(maxMemory int64) Option {
	return func(opts *Options) {
		opts.MaxMemory = maxMemory
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
	CodeUnknownField IssueCode = "unknownField"
	CodePathConflict IssueCode = "pathConflict"
	CodeInvalidJSON  IssueCode = "invalidJSON"
	CodeRequired     IssueCode = "required"
	CodeType         IssueCode = "type"
	CodeEnum         IssueCode = "enum"
	CodeMin          IssueCode = "min"
	CodeMax          IssueCode = "max"
	CodeMinLength    IssueCode = "minLength"
	CodeMaxLength    IssueCode = "maxLength"
	CodePattern      IssueCode = "pattern"
	CodeMinItems     IssueCode = "minItems"
	CodeMaxItems     IssueCode = "maxItems"
	CodeObject       IssueCode = "object"
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
