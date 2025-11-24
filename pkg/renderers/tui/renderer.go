package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/render"
)

// Renderer implements render.Renderer for terminal-driven sessions. It is
// scaffolded here; interaction logic arrives in later phases per TUI_TDD.
type Renderer struct {
	driver            PromptDriver
	outputFormat      OutputFormat
	httpClient        *http.Client
	submitTransformer SubmitTransformer
	theme             Theme
}

// New constructs a TUI renderer with defaults (survey driver, JSON output).
func New(options ...Option) (render.Renderer, error) {
	driver, err := newSurveyDriver()
	if err != nil {
		return nil, err
	}

	r := &Renderer{
		driver:       driver,
		outputFormat: OutputFormatJSON,
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(r)
	}

	if r.driver == nil {
		r.driver, err = newSurveyDriver()
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Name reports the renderer identifier.
func (r *Renderer) Name() string {
	return "tui"
}

// ContentType reports the serialization format used by Render.
func (r *Renderer) ContentType() string {
	switch r.outputFormat {
	case OutputFormatFormURLEncoded:
		return "application/x-www-form-urlencoded"
	case OutputFormatPrettyText:
		return "text/plain"
	default:
		return "application/json"
	}
}

// Render will orchestrate prompts and collect values in later phases. For now
// it enforces basic preconditions and signals lack of implementation.
func (r *Renderer) Render(ctx context.Context, form model.FormModel, opts render.RenderOptions) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("tui: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.driver == nil {
		return nil, errors.New("tui: prompt driver is nil")
	}

	state := NewState(opts.Values, opts.Errors)
	rulesCache := make(map[string]validationRules)
	relCache := make(map[string][]relOption)

	for _, field := range form.Fields {
		if err := r.promptField(ctx, field, field.Name, state, rulesCache, relCache); err != nil {
			return nil, err
		}
	}

	values := state.Values()
	if r.submitTransformer != nil {
		var err error
		values, err = r.submitTransformer(values)
		if err != nil {
			return nil, fmt.Errorf("tui: submit transformer: %w", err)
		}
	}

	return r.serialize(values)
}

func (r *Renderer) promptField(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules, relCache map[string][]relOption) error {
	if field.Relationship != nil {
		return r.promptRelationship(ctx, field, path, state, rulesCache, relCache)
	}
	switch field.Type {
	case model.FieldTypeBoolean:
		return r.promptBoolean(ctx, field, path, state, rulesCache)
	case model.FieldTypeInteger, model.FieldTypeNumber:
		return r.promptNumber(ctx, field, path, state, rulesCache)
	case model.FieldTypeArray:
		return r.promptArray(ctx, field, path, state, rulesCache, relCache)
	case model.FieldTypeObject:
		return r.promptObject(ctx, field, path, state, rulesCache, relCache)
	default:
		// string and fallbacks
		if len(field.Enum) > 0 {
			return r.promptEnum(ctx, field, path, state, rulesCache)
		}
		return r.promptString(ctx, field, path, state, rulesCache)
	}
}

func (r *Renderer) promptString(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules) error {
	label := displayLabel(field)
	help := displayHelp(field)

	rules := collectValidationRules(field, rulesCache)
	defaultVal := defaultStringValue(state, path, field.Default)

	usePassword := field.Format == "password" || strings.EqualFold(field.Metadata["cli.secret"], "true")
	isTextArea := field.Format == "textarea" || field.UIHints["input"] == "textarea"

	for {
		if !rules.required && defaultVal == "" && rulesCache != nil {
			// allow skip by entering empty; prompt still shown once
		}

		var response string
		var err error
		cfg := InputConfig{
			Message: label,
			Default: defaultVal,
			Help:    help,
		}
		if usePassword {
			response, err = r.driver.Password(ctx, cfg)
		} else if isTextArea {
			response, err = r.driver.TextArea(ctx, TextAreaConfig{
				Message: label,
				Default: defaultVal,
				Help:    help,
			})
		} else {
			response, err = r.driver.Input(ctx, cfg)
		}
		if err != nil {
			return err
		}

		if !rules.required && strings.TrimSpace(response) == "" {
			_ = state.SetValue(path, response)
			return nil
		}

		if err := rules.validateString(response); err != nil {
			_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
			continue
		}

		_ = state.SetValue(path, response)
		return nil
	}
}

func (r *Renderer) promptBoolean(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules) error {
	label := displayLabel(field)
	help := displayHelp(field)
	rules := collectValidationRules(field, rulesCache)
	defaultVal := defaultBoolValue(state, path, field.Default)

	for {
		resp, err := r.driver.Confirm(ctx, ConfirmConfig{
			Message: label,
			Default: defaultVal,
			Help:    help,
		})
		if err != nil {
			return err
		}
		if err := rules.validateBool(resp); err != nil {
			_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
			continue
		}
		_ = state.SetValue(path, resp)
		return nil
	}
}

func (r *Renderer) promptNumber(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules) error {
	label := displayLabel(field)
	help := displayHelp(field)
	rules := collectValidationRules(field, rulesCache)
	defaultVal, hasDefault := defaultNumberValue(state, path, field.Default, field.Type == model.FieldTypeInteger)
	defaultStr := ""
	if hasDefault {
		defaultStr = fmt.Sprint(defaultVal)
	}

	for {
		input, err := r.driver.Input(ctx, InputConfig{
			Message: label,
			Default: defaultStr,
			Help:    help,
		})
		if err != nil {
			return err
		}

		if input == "" {
			if rules.required {
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: required", path))
				continue
			}
			_ = state.SetValue(path, nil)
			return nil
		}

		var parsed any
		if field.Type == model.FieldTypeInteger {
			i, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
			if err != nil {
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
				continue
			}
			parsed = i
		} else {
			f, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
			if err != nil {
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
				continue
			}
			parsed = f
		}

		if err := rules.validateNumber(parsed); err != nil {
			_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
			continue
		}

		_ = state.SetValue(path, parsed)
		return nil
	}
}

func (r *Renderer) promptEnum(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules) error {
	label := displayLabel(field)
	help := displayHelp(field)
	rules := collectValidationRules(field, rulesCache)

	options := stringifyEnum(field.Enum)
	defaultIdx := -1

	if v, ok := state.GetValue(path); ok {
		if s, ok := v.(string); ok {
			defaultIdx = indexOf(options, s)
		}
	}

	for {
		idx, err := r.driver.Select(ctx, SelectConfig{
			Message:      label,
			Options:      options,
			DefaultIndex: defaultIdx,
			Help:         help,
		})
		if err != nil {
			return err
		}
		if idx < 0 || idx >= len(options) {
			_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s selection", path))
			continue
		}
		selected := options[idx]
		if err := rules.validateString(selected); err != nil {
			_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
			continue
		}
		_ = state.SetValue(path, selected)
		return nil
	}
}

func (r *Renderer) promptArray(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules, relCache map[string][]relOption) error {
	// Enum-backed array -> multiselect of known options
	if len(field.Enum) > 0 {
		label := displayLabel(field)
		help := displayHelp(field)
		rules := collectValidationRules(field, rulesCache)
		options := stringifyEnum(field.Enum)
		defaults := indicesOf(options, stringifySlice(getArrayValue(state, path)))

		for {
			indices, err := r.driver.MultiSelect(ctx, SelectConfig{
				Message:  label,
				Options:  options,
				Defaults: defaults,
				Help:     help,
			})
			if err != nil {
				return err
			}
			selected := valuesFromIndices(options, indices)
			if err := rules.validateArray(selected); err != nil {
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
				continue
			}
			_ = state.SetValue(path, toAnySlice(selected))
			return nil
		}
	}

	// Non-enum arrays: prompt items sequentially.
	rules := collectValidationRules(field, rulesCache)
	var items []any

	// seed existing values if present
	if existing, ok := state.GetValue(path); ok {
		items = append(items, coerceAnySlice(existing)...)
	}

	// allow skipping when not required and no prefill
	if len(items) == 0 && !rules.required {
		add, err := r.driver.Confirm(ctx, ConfirmConfig{
			Message: "Add items?",
			Default: false,
		})
		if err != nil {
			return err
		}
		if !add {
			return state.SetValue(path, items)
		}
	}

	// prompt loop
	for {
		idx := len(items)
		itemPath := fmt.Sprintf("%s.%d", path, idx)
		if field.Items == nil {
			return fmt.Errorf("tui: array field %s missing items schema", path)
		}
		if err := r.promptField(ctx, *field.Items, itemPath, state, rulesCache, relCache); err != nil {
			return err
		}
		val, _ := state.GetValue(itemPath)
		items = append(items, val)

		// ask to continue
		more, err := r.driver.Confirm(ctx, ConfirmConfig{
			Message: "Add another?",
			Default: false,
		})
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}

	if err := rules.validateArray(items); err != nil {
		return err
	}
	return state.SetValue(path, items)
}

func (r *Renderer) promptObject(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules, relCache map[string][]relOption) error {
	for _, child := range field.Nested {
		childPath := fmt.Sprintf("%s.%s", path, child.Name)
		if err := r.promptField(ctx, child, childPath, state, rulesCache, relCache); err != nil {
			return err
		}
	}
	return nil
}

func (r *Renderer) serialize(values map[string]any) ([]byte, error) {
	switch r.outputFormat {
	case OutputFormatFormURLEncoded:
		return []byte(flattenForm(values)), nil
	case OutputFormatPrettyText:
		return []byte(prettyPrint(values)), nil
	default:
		return jsonBytes(values)
	}
}

func displayLabel(field model.Field) string {
	if field.Label != "" {
		return field.Label
	}
	return field.Name
}

func displayHelp(field model.Field) string {
	if h := field.Metadata["cli.help"]; h != "" {
		return h
	}
	return field.Description
}

func stringifyEnum(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprint(v))
	}
	return out
}

func valuesFromIndices(options []string, indices []int) []string {
	out := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(options) {
			out = append(out, options[idx])
		}
	}
	return out
}

func stringifySlice(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprint(v))
	}
	return out
}

func toAnySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

func coerceAnySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	default:
		return nil
	}
}

func getArrayValue(state *State, path string) []any {
	if v, ok := state.GetValue(path); ok {
		if arr, ok := v.([]any); ok {
			return arr
		}
	}
	return nil
}

func defaultStringValue(state *State, path string, def any) string {
	if v, ok := state.GetValue(path); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if s, ok := def.(string); ok {
		return s
	}
	return ""
}

func defaultBoolValue(state *State, path string, def any) bool {
	if v, ok := state.GetValue(path); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	if b, ok := def.(bool); ok {
		return b
	}
	return false
}

func defaultNumberValue(state *State, path string, def any, integer bool) (any, bool) {
	if v, ok := state.GetValue(path); ok {
		switch t := v.(type) {
		case int, int64, float64:
			return t, true
		}
	}
	switch t := def.(type) {
	case int, int64, float64:
		if integer {
			switch num := t.(type) {
			case int:
				return int64(num), true
			case int64:
				return num, true
			case float64:
				return int64(num), true
			}
		}
		return t, true
	}
	return nil, false
}

type validationRules struct {
	required bool
	min      *float64
	max      *float64
	minLen   *int
	maxLen   *int
	pattern  *regexp.Regexp
}

type relConfig struct {
	url        string
	method     string
	labelField string
	valueField string
	results    string
	params     map[string]string
}

type relOption struct {
	Label string
	Value string
}

func (r *Renderer) promptRelationship(ctx context.Context, field model.Field, path string, state *State, rulesCache map[string]validationRules, relCache map[string][]relOption) error {
	rel := field.Relationship
	if rel == nil {
		return errors.New("tui: relationship metadata missing")
	}

	label := displayLabel(field)
	help := displayHelp(field)
	rules := collectValidationRules(field, rulesCache)

	cfg, hasEndpoint := parseRelConfig(field.Metadata)
	options := r.relationshipOptions(ctx, cfg, relCache)

	// apply relationship.current when state lacks value
	if _, ok := state.GetValue(path); !ok {
		if current := parseRelationshipCurrent(field.Metadata); len(current) > 0 {
			if rel.Kind == model.RelationshipHasMany || strings.EqualFold(rel.Cardinality, "many") || field.Type == model.FieldTypeArray {
				_ = state.SetValue(path, toAnySlice(current))
			} else {
				_ = state.SetValue(path, current[0])
			}
		}
	}

	isMany := rel.Kind == model.RelationshipHasMany || strings.EqualFold(rel.Cardinality, "many") || field.Type == model.FieldTypeArray

	if isMany {
		// use multiselect if options available, else manual id input loop
		if len(options) > 0 {
			defaults := indicesOfValues(options, stringifySlice(getArrayValue(state, path)))
			for {
				indices, err := r.driver.MultiSelect(ctx, SelectConfig{
					Message:  label,
					Options:  labels(options),
					Defaults: defaults,
					Help:     help,
				})
				if err != nil {
					return err
				}
				selected := valuesFromOptionIndices(options, indices)
				if err := rules.validateArray(toAnySlice(selected)); err != nil {
					_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
					continue
				}
				return state.SetValue(path, toAnySlice(selected))
			}
		}

		// manual entry when no options
		var entries []string
		if existing := getArrayValue(state, path); len(existing) > 0 {
			entries = stringifySlice(existing)
		}

		if len(entries) == 0 && rules.required {
			_ = r.driver.Info(ctx, fmt.Sprintf("%s is required; enter at least one id", label))
		}

		for {
			val, err := r.driver.Input(ctx, InputConfig{
				Message: fmt.Sprintf("%s id", label),
				Help:    help,
			})
			if err != nil {
				return err
			}
			val = strings.TrimSpace(val)
			if val != "" {
				entries = append(entries, val)
			}

			if err := rules.validateArray(toAnySlice(entries)); err != nil {
				// If required and empty, keep prompting.
				if errors.Is(err, ErrAborted) {
					return err
				}
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
			}

			more, err := r.driver.Confirm(ctx, ConfirmConfig{
				Message: "Add another?",
				Default: false,
			})
			if err != nil {
				return err
			}
			if !more {
				if err := rules.validateArray(toAnySlice(entries)); err != nil {
					_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
					continue
				}
				return state.SetValue(path, toAnySlice(entries))
			}
		}
	}

	// Single-cardinality
	if len(options) > 0 {
		defaultVal := defaultStringValue(state, path, nil)
		defaultIdx := indexOfValue(options, defaultVal)
		for {
			idx, err := r.driver.Select(ctx, SelectConfig{
				Message:      label,
				Options:      labels(options),
				DefaultIndex: defaultIdx,
				Help:         help,
			})
			if err != nil {
				return err
			}
			if idx < 0 || idx >= len(options) {
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s selection", path))
				continue
			}
			selected := options[idx].Value
			if err := rules.validateString(selected); err != nil {
				_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
				continue
			}
			return state.SetValue(path, selected)
		}
	}

	// Fallback to plain input when no options
	for {
		val, err := r.driver.Input(ctx, InputConfig{
			Message: label,
			Help:    help,
		})
		if err != nil {
			return err
		}
		val = strings.TrimSpace(val)
		if err := rules.validateString(val); err != nil {
			_ = r.driver.Info(ctx, fmt.Sprintf("Invalid %s: %v", path, err))
			continue
		}
		return state.SetValue(path, val)
	}
}

func labels(opts []relOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		if o.Label != "" {
			out[i] = o.Label
		} else {
			out[i] = o.Value
		}
	}
	return out
}

func indexOfValue(opts []relOption, value string) int {
	if value == "" {
		return -1
	}
	for i, o := range opts {
		if o.Value == value {
			return i
		}
	}
	return -1
}

func indicesOfValues(opts []relOption, values []string) []int {
	if len(values) == 0 {
		return nil
	}
	valueSet := make(map[string]struct{}, len(values))
	for _, v := range values {
		valueSet[v] = struct{}{}
	}
	var out []int
	for i, o := range opts {
		if _, ok := valueSet[o.Value]; ok {
			out = append(out, i)
		}
	}
	return out
}

func valuesFromOptionIndices(opts []relOption, indices []int) []string {
	out := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(opts) {
			out = append(out, opts[idx].Value)
		}
	}
	return out
}

func parseRelationshipCurrent(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	raw := strings.TrimSpace(metadata["relationship.current"])
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var list []string
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			return list
		}
	}
	return []string{raw}
}

func parseRelConfig(metadata map[string]string) (relConfig, bool) {
	if len(metadata) == 0 {
		return relConfig{}, false
	}
	url := strings.TrimSpace(metadata["relationship.endpoint.url"])
	if url == "" {
		return relConfig{}, false
	}
	cfg := relConfig{
		url:        url,
		method:     strings.ToUpper(strings.TrimSpace(metadata["relationship.endpoint.method"])),
		labelField: strings.TrimSpace(metadata["relationship.endpoint.labelField"]),
		valueField: strings.TrimSpace(metadata["relationship.endpoint.valueField"]),
		results:    strings.TrimSpace(metadata["relationship.endpoint.resultsPath"]),
		params:     map[string]string{},
	}
	if cfg.method == "" {
		cfg.method = http.MethodGet
	}
	for key, value := range metadata {
		if !strings.HasPrefix(key, "relationship.endpoint.params.") {
			continue
		}
		param := strings.TrimPrefix(key, "relationship.endpoint.params.")
		if param == "" || strings.TrimSpace(value) == "" {
			continue
		}
		cfg.params[param] = value
	}
	return cfg, true
}

func (r *Renderer) relationshipOptions(ctx context.Context, cfg relConfig, cache map[string][]relOption) []relOption {
	// Enum or missing config handled before calling this; http optional.
	if cfg.url == "" || r.httpClient == nil {
		return nil
	}

	cacheKey := cacheKeyFor(cfg)
	if opts, ok := cache[cacheKey]; ok {
		return opts
	}

	opts, err := r.fetchRelationshipOptions(ctx, cfg)
	if err != nil {
		_ = r.driver.Info(ctx, fmt.Sprintf("Warning: relationship options fetch failed (%v); falling back to manual input", err))
		cache[cacheKey] = nil
		return nil
	}

	cache[cacheKey] = opts
	return opts
}

func cacheKeyFor(cfg relConfig) string {
	var b strings.Builder
	b.WriteString(cfg.method)
	b.WriteString(" ")
	b.WriteString(cfg.url)
	if len(cfg.params) > 0 {
		keys := make([]string, 0, len(cfg.params))
		for k := range cfg.params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(";")
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(cfg.params[k])
		}
	}
	return b.String()
}

func (r *Renderer) fetchRelationshipOptions(ctx context.Context, cfg relConfig) ([]relOption, error) {
	reqURL, err := url.Parse(cfg.url)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := reqURL.Query()
	for k, v := range cfg.params {
		q.Set(k, v)
	}
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, cfg.method, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	items := extractResults(payload, cfg.results)
	if len(items) == 0 {
		return nil, nil
	}

	var opts []relOption
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		val := pickValue(obj, cfg.valueField)
		lbl := pickValue(obj, cfg.labelField)
		if val == "" {
			continue
		}
		if lbl == "" {
			lbl = val
		}
		opts = append(opts, relOption{
			Label: lbl,
			Value: val,
		})
	}
	return opts, nil
}

func extractResults(payload any, path string) []any {
	if payload == nil {
		return nil
	}
	cur := payload
	if path != "" {
		for _, segment := range strings.Split(path, ".") {
			switch node := cur.(type) {
			case map[string]any:
				cur = node[segment]
			default:
				return nil
			}
		}
	}
	switch v := cur.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = v[i]
		}
		return out
	default:
		return nil
	}
}

func pickValue(m map[string]any, path string) string {
	if path == "" {
		return ""
	}
	cur := any(m)
	for _, segment := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[segment]
		default:
			return ""
		}
	}
	return fmt.Sprint(cur)
}

func collectValidationRules(field model.Field, cache map[string]validationRules) validationRules {
	if rules, ok := cache[field.Name]; ok {
		return rules
	}
	rules := validationRules{required: field.Required}
	for _, v := range field.Validations {
		switch v.Kind {
		case model.ValidationRuleMin:
			if val, ok := parseFloat(v.Params["value"]); ok {
				rules.min = &val
			}
		case model.ValidationRuleMax:
			if val, ok := parseFloat(v.Params["value"]); ok {
				rules.max = &val
			}
		case model.ValidationRuleMinLength:
			if val, ok := parseInt(v.Params["value"]); ok {
				rules.minLen = &val
			}
		case model.ValidationRuleMaxLength:
			if val, ok := parseInt(v.Params["value"]); ok {
				rules.maxLen = &val
			}
		case model.ValidationRulePattern:
			if expr := v.Params["pattern"]; expr != "" {
				if re, err := regexp.Compile(expr); err == nil {
					rules.pattern = re
				}
			}
		}
	}
	cache[field.Name] = rules
	return rules
}

func (r validationRules) validateString(value string) error {
	if r.required && strings.TrimSpace(value) == "" {
		return errors.New("required")
	}
	if r.minLen != nil && len(value) < *r.minLen {
		return fmt.Errorf("min length %d", *r.minLen)
	}
	if r.maxLen != nil && len(value) > *r.maxLen {
		return fmt.Errorf("max length %d", *r.maxLen)
	}
	if r.pattern != nil && !r.pattern.MatchString(value) {
		return errors.New("does not match required pattern")
	}
	return nil
}

func (r validationRules) validateBool(value bool) error {
	if r.required {
		// bool is always set; nothing to do
	}
	return nil
}

func (r validationRules) validateNumber(value any) error {
	var v float64
	switch n := value.(type) {
	case int:
		v = float64(n)
	case int64:
		v = float64(n)
	case float64:
		v = n
	default:
		return fmt.Errorf("expected number, got %T", value)
	}
	if r.min != nil && v < *r.min {
		return fmt.Errorf("min %v", *r.min)
	}
	if r.max != nil && v > *r.max {
		return fmt.Errorf("max %v", *r.max)
	}
	return nil
}

func (r validationRules) validateArray(value []any) error {
	if r.required && len(value) == 0 {
		return errors.New("required")
	}
	if r.minLen != nil && len(value) < *r.minLen {
		return fmt.Errorf("min length %d", *r.minLen)
	}
	if r.maxLen != nil && len(value) > *r.maxLen {
		return fmt.Errorf("max length %d", *r.maxLen)
	}
	return nil
}

func parseFloat(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	val, err := strconv.ParseFloat(raw, 64)
	return val, err == nil
}

func parseInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	val, err := strconv.Atoi(raw)
	return val, err == nil
}

func flattenForm(values map[string]any) string {
	flattened := url.Values{}
	flatten("", values, flattened)
	return flattened.Encode()
}

func flatten(prefix string, value any, out url.Values) {
	switch v := value.(type) {
	case map[string]any:
		for key, val := range v {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flatten(next, val, out)
		}
	case []any:
		for _, val := range v {
			out.Add(prefix+"[]", fmt.Sprint(val))
		}
	default:
		out.Set(prefix, fmt.Sprint(v))
	}
}

func prettyPrint(values map[string]any) string {
	var b strings.Builder
	writePretty(&b, "", values)
	return b.String()
}

func writePretty(b *strings.Builder, prefix string, value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, val := range v {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			writePretty(b, next, val)
		}
	case []any:
		for idx, val := range v {
			next := fmt.Sprintf("%s[%d]", prefix, idx)
			writePretty(b, next, val)
		}
	default:
		if prefix != "" {
			fmt.Fprintf(b, "%s=%v\n", prefix, v)
		}
	}
}

func jsonBytes(values map[string]any) ([]byte, error) {
	return json.Marshal(values)
}
