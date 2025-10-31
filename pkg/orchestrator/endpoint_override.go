package orchestrator

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
)

// EndpointConfig mirrors the x-endpoint extension contract documented in
// JS_TDD.md §5.1. Zero values are omitted when converted to metadata.
type EndpointConfig struct {
	URL           string
	Method        string
	LabelField    string
	ValueField    string
	ResultsPath   string
	Params        map[string]string
	DynamicParams map[string]string
	Mapping       EndpointMapping
	Auth          *EndpointAuth
	SubmitAs      string
}

// EndpointMapping remaps response payload structures (value/label paths).
type EndpointMapping struct {
	Value string
	Label string
}

// EndpointAuth describes how runtime helpers should supply authentication
// tokens when resolving relationship options.
type EndpointAuth struct {
	Strategy string
	Header   string
	Source   string
}

// EndpointOverride allows callers to provide endpoint metadata when an OpenAPI
// schema omits x-endpoint extensions for a relationship field.
type EndpointOverride struct {
	OperationID string
	FieldPath   string
	Endpoint    EndpointConfig
}

// WithEndpointOverrides registers endpoint overrides that run after the model
// builder executes. Overrides are scoped per operation and only applied when
// the target field lacks endpoint metadata.
func WithEndpointOverrides(overrides []EndpointOverride) Option {
	cloned := cloneEndpointOverrides(overrides)
	return func(o *Orchestrator) {
		if len(cloned) == 0 || o == nil {
			return
		}

		if o.endpointOverrides == nil {
			o.endpointOverrides = make(map[string][]EndpointOverride)
		}

		for _, override := range cloned {
			if err := validateEndpointOverride(override); err != nil {
				o.initialiseErr = appendInitialiseError(o.initialiseErr, err)
				continue
			}
			o.endpointOverrides[override.OperationID] = append(o.endpointOverrides[override.OperationID], override)
		}
	}
}

func cloneEndpointOverrides(overrides []EndpointOverride) []EndpointOverride {
	if len(overrides) == 0 {
		return nil
	}
	cloned := make([]EndpointOverride, 0, len(overrides))
	for _, override := range overrides {
		copied := override
		if len(override.Endpoint.Params) > 0 {
			copied.Endpoint.Params = cloneStringMap(override.Endpoint.Params)
		}
		if len(override.Endpoint.DynamicParams) > 0 {
			copied.Endpoint.DynamicParams = cloneStringMap(override.Endpoint.DynamicParams)
		}
		if override.Endpoint.Auth != nil {
			auth := *override.Endpoint.Auth
			copied.Endpoint.Auth = &auth
		}
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func validateEndpointOverride(override EndpointOverride) error {
	if strings.TrimSpace(override.OperationID) == "" {
		return errors.New("orchestrator: endpoint override missing operation id")
	}
	if strings.TrimSpace(override.FieldPath) == "" {
		return fmt.Errorf("orchestrator: endpoint override %q missing field path", override.OperationID)
	}
	if strings.TrimSpace(override.Endpoint.URL) == "" {
		return fmt.Errorf("orchestrator: endpoint override %q for %s missing endpoint url", override.OperationID, override.FieldPath)
	}
	return nil
}

func appendInitialiseError(existing, next error) error {
	if existing == nil {
		return next
	}
	return fmt.Errorf("%v; %w", existing, next)
}

func (o *Orchestrator) applyEndpointOverrides(operationID string, form *pkgmodel.FormModel) {
	if form == nil || len(o.endpointOverrides) == 0 {
		return
	}
	overrides := o.endpointOverrides[operationID]
	if len(overrides) == 0 {
		return
	}

	for _, override := range overrides {
		target := locateField(form.Fields, strings.Split(override.FieldPath, "."))
		if target == nil {
			continue
		}
		if hasEndpointMetadata(target.Metadata) {
			continue
		}
		metadata := flattenEndpointConfig(override.Endpoint)
		if len(metadata) == 0 {
			continue
		}
		if target.Metadata == nil {
			target.Metadata = make(map[string]string, len(metadata))
		}
		for key, value := range metadata {
			target.Metadata[key] = value
		}
	}
}

func locateField(fields []pkgmodel.Field, segments []string) *pkgmodel.Field {
	if len(segments) == 0 {
		return nil
	}
	for i := range fields {
		field := &fields[i]
		if field.Name != segments[0] {
			continue
		}
		if len(segments) == 1 {
			return field
		}
		if nested := locateField(field.Nested, segments[1:]); nested != nil {
			return nested
		}
		if nested := locateFieldInItem(field.Items, segments[1:]); nested != nil {
			return nested
		}
	}
	return nil
}

func locateFieldInItem(item *pkgmodel.Field, segments []string) *pkgmodel.Field {
	if item == nil {
		return nil
	}
	if len(segments) == 0 {
		return item
	}
	if item.Name == segments[0] {
		if len(segments) == 1 {
			return item
		}
		if nested := locateField(item.Nested, segments[1:]); nested != nil {
			return nested
		}
		if nested := locateFieldInItem(item.Items, segments[1:]); nested != nil {
			return nested
		}
		return nil
	}
	if nested := locateField(item.Nested, segments); nested != nil {
		return nested
	}
	return locateFieldInItem(item.Items, segments)
}

func hasEndpointMetadata(metadata map[string]string) bool {
	if len(metadata) == 0 {
		return false
	}
	if _, ok := metadata["relationship.endpoint.url"]; ok {
		return true
	}
	for key := range metadata {
		if strings.HasPrefix(key, "relationship.endpoint.") {
			return true
		}
	}
	return false
}

func flattenEndpointConfig(cfg EndpointConfig) map[string]string {
	meta := make(map[string]string)

	add := func(key, value string) {
		if value == "" {
			return
		}
		meta[key] = value
	}

	add("relationship.endpoint.url", strings.TrimSpace(cfg.URL))
	if cfg.Method != "" {
		add("relationship.endpoint.method", strings.ToUpper(cfg.Method))
	}
	add("relationship.endpoint.labelField", strings.TrimSpace(cfg.LabelField))
	add("relationship.endpoint.valueField", strings.TrimSpace(cfg.ValueField))
	add("relationship.endpoint.resultsPath", strings.TrimSpace(cfg.ResultsPath))
	add("relationship.endpoint.submitAs", strings.TrimSpace(cfg.SubmitAs))

	if len(cfg.Params) > 0 {
		for _, key := range sortedKeys(cfg.Params) {
			add("relationship.endpoint.params."+key, cfg.Params[key])
		}
	}
	if len(cfg.DynamicParams) > 0 {
		for _, key := range sortedKeys(cfg.DynamicParams) {
			add("relationship.endpoint.dynamicParams."+key, cfg.DynamicParams[key])
		}
		if refs := extractFieldReferences(cfg.DynamicParams); len(refs) > 0 {
			add("relationship.endpoint.refreshOn", strings.Join(refs, ","))
		}
	}

	if cfg.Mapping.Value != "" {
		add("relationship.endpoint.mapping.value", cfg.Mapping.Value)
	}
	if cfg.Mapping.Label != "" {
		add("relationship.endpoint.mapping.label", cfg.Mapping.Label)
	}

	if cfg.Auth != nil {
		add("relationship.endpoint.auth.strategy", strings.TrimSpace(cfg.Auth.Strategy))
		add("relationship.endpoint.auth.header", strings.TrimSpace(cfg.Auth.Header))
		add("relationship.endpoint.auth.source", strings.TrimSpace(cfg.Auth.Source))
	}

	if len(meta) == 0 {
		return nil
	}
	return meta
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var fieldPlaceholderPattern = regexp.MustCompile(`\{\{\s*field:([^\}\s]+)\s*\}\}`)

func extractFieldReferences(params map[string]string) []string {
	if len(params) == 0 {
		return nil
	}
	result := make(map[string]struct{})
	for _, value := range params {
		for _, match := range fieldPlaceholderPattern.FindAllStringSubmatch(value, -1) {
			if len(match) < 2 {
				continue
			}
			name := strings.TrimSpace(match[1])
			if name == "" {
				continue
			}
			result[name] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	out := make([]string, 0, len(result))
	for name := range result {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
