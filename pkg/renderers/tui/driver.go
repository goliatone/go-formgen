package tui

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// InputConfig configures a basic text input prompt.
type InputConfig struct {
	Message     string
	Default     string
	Help        string
	Placeholder string
	Validator   func(string) error
}

// ConfirmConfig configures a yes/no style prompt.
type ConfirmConfig struct {
	Message string
	Default bool
	Help    string
}

// SelectConfig configures a single or multi-select prompt.
type SelectConfig struct {
	Message      string
	Options      []string
	DefaultIndex int
	Defaults     []int // used for multi-select; indices into Options
	Help         string
	PageSize     int
}

// TextAreaConfig configures a multi-line text prompt.
type TextAreaConfig struct {
	Message string
	Default string
	Help    string
}

// RepeatConfig configures repeating prompts for array/object editing flows.
type RepeatConfig struct {
	Message string
	Help    string
}

// PromptDriver abstracts the actual TUI implementation so render logic can be
// tested without a real terminal and callers can swap implementations.
type PromptDriver interface {
	Input(ctx context.Context, cfg InputConfig) (string, error)
	Password(ctx context.Context, cfg InputConfig) (string, error)
	Confirm(ctx context.Context, cfg ConfirmConfig) (bool, error)
	Select(ctx context.Context, cfg SelectConfig) (int, error)
	MultiSelect(ctx context.Context, cfg SelectConfig) ([]int, error)
	TextArea(ctx context.Context, cfg TextAreaConfig) (string, error)
	Repeat(ctx context.Context, cfg RepeatConfig) ([][]byte, error)
	Info(ctx context.Context, msg string) error
}

type surveyDriver struct{}

func newSurveyDriver() (PromptDriver, error) {
	return &surveyDriver{}, nil
}

func (d *surveyDriver) Input(ctx context.Context, cfg InputConfig) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var out string
	prompt := &survey.Input{
		Message: cfg.Message,
		Help:    cfg.Help,
		Default: cfg.Default,
	}
	var opts []survey.AskOpt
	if cfg.Validator != nil {
		opts = append(opts, survey.WithValidator(cfg.Validator))
	}
	if err := survey.AskOne(prompt, &out, opts...); err != nil {
		return "", translateSurveyErr(err)
	}
	return out, nil
}

func (d *surveyDriver) Password(ctx context.Context, cfg InputConfig) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var out string
	prompt := &survey.Password{
		Message: cfg.Message,
		Help:    cfg.Help,
		Default: cfg.Default,
	}
	var opts []survey.AskOpt
	if cfg.Validator != nil {
		opts = append(opts, survey.WithValidator(cfg.Validator))
	}
	if err := survey.AskOne(prompt, &out, opts...); err != nil {
		return "", translateSurveyErr(err)
	}
	return out, nil
}

func (d *surveyDriver) Confirm(ctx context.Context, cfg ConfirmConfig) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var out bool
	prompt := &survey.Confirm{
		Message: cfg.Message,
		Help:    cfg.Help,
		Default: cfg.Default,
	}
	if err := survey.AskOne(prompt, &out); err != nil {
		return false, translateSurveyErr(err)
	}
	return out, nil
}

func (d *surveyDriver) Select(ctx context.Context, cfg SelectConfig) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var out string
	prompt := &survey.Select{
		Message: cfg.Message,
		Options: cfg.Options,
		Help:    cfg.Help,
	}
	if cfg.PageSize > 0 {
		prompt.PageSize = cfg.PageSize
	}
	if cfg.DefaultIndex >= 0 && cfg.DefaultIndex < len(cfg.Options) {
		prompt.Default = cfg.Options[cfg.DefaultIndex]
	}
	if err := survey.AskOne(prompt, &out); err != nil {
		return 0, translateSurveyErr(err)
	}
	return indexOf(cfg.Options, out), nil
}

func (d *surveyDriver) MultiSelect(ctx context.Context, cfg SelectConfig) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []string
	prompt := &survey.MultiSelect{
		Message: cfg.Message,
		Options: cfg.Options,
		Help:    cfg.Help,
	}
	if cfg.PageSize > 0 {
		prompt.PageSize = cfg.PageSize
	}
	if len(cfg.Defaults) > 0 {
		prompt.Default = defaultsFromIndices(cfg.Options, cfg.Defaults)
	}
	if err := survey.AskOne(prompt, &out); err != nil {
		return nil, translateSurveyErr(err)
	}
	return indicesOf(cfg.Options, out), nil
}

func (d *surveyDriver) TextArea(ctx context.Context, cfg TextAreaConfig) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var out string
	prompt := &survey.Multiline{
		Message: cfg.Message,
		Help:    cfg.Help,
		Default: cfg.Default,
	}
	if err := survey.AskOne(prompt, &out); err != nil {
		return "", translateSurveyErr(err)
	}
	return out, nil
}

func (d *surveyDriver) Repeat(ctx context.Context, cfg RepeatConfig) ([][]byte, error) {
	// Placeholder until repeat flows are handled explicitly in renderer logic.
	return nil, ErrRepeatUnsupported
}

func (d *surveyDriver) Info(ctx context.Context, msg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stdout, msg)
	return err
}

func translateSurveyErr(err error) error {
	if errors.Is(err, terminal.InterruptErr) {
		return ErrAborted
	}
	return err
}

func indexOf(options []string, value string) int {
	for i, option := range options {
		if option == value {
			return i
		}
	}
	return -1
}

func indicesOf(options, values []string) []int {
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		seen[v] = struct{}{}
	}
	var out []int
	for i, option := range options {
		if _, ok := seen[option]; ok {
			out = append(out, i)
		}
	}
	return out
}

func defaultsFromIndices(options []string, indices []int) []string {
	var out []string
	for _, idx := range indices {
		if idx >= 0 && idx < len(options) {
			out = append(out, options[idx])
		}
	}
	return out
}
