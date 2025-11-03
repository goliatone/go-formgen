package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

const (
	extensionNamespace       = "x-formgen"
	endpointExtensionKey     = "x-endpoint"
	currentValueExtensionKey = "x-current-value"
)

// Builder converts OpenAPI operations into form models.
type Builder struct {
	opts Options
}

// New creates a Builder with the supplied options.
func New(options Options) *Builder {
	opts := defaultOptions()
	if options.Labeler != nil {
		opts.Labeler = options.Labeler
	}
	return &Builder{opts: opts}
}

// Build transforms an OpenAPI operation into a FormModel suitable for
// rendering. It focuses on request bodies and metadata as described in the
// README.
func (b *Builder) Build(op pkgopenapi.Operation) (FormModel, error) {
	if err := validateOperation(op); err != nil {
		return FormModel{}, err
	}

	form := FormModel{
		OperationID: op.ID,
		Endpoint:    op.Path,
		Method:      strings.ToUpper(op.Method),
		Summary:     op.Summary,
		Description: op.Description,
	}

	if form.Metadata == nil {
		form.Metadata = make(map[string]string)
	}
	if op.Summary != "" {
		form.Metadata["summary"] = op.Summary
	}
	if op.Description != "" {
		form.Metadata["description"] = op.Description
	}
	formExt := metadataFromExtensions(op.Extensions)
	bodyExt := metadataFromExtensions(op.RequestBody.Extensions)
	mergeMetadata(form.Metadata, formExt)
	mergeMetadata(form.Metadata, bodyExt)
	form.UIHints = mergeUIHints(form.UIHints, filterUIHints(formExt))
	form.UIHints = mergeUIHints(form.UIHints, filterUIHints(bodyExt))

	fields, err := b.fieldsFromSchema("", op.RequestBody, true)
	if err != nil {
		return FormModel{}, err
	}
	form.Fields = fields

	if len(form.Metadata) == 0 {
		form.Metadata = nil
	}
	if len(form.UIHints) == 0 {
		form.UIHints = nil
	}

	return form, nil
}

func (b *Builder) fieldsFromSchema(name string, schema pkgopenapi.Schema, required bool) ([]Field, error) {
	if schema.Ref != "" && schema.Type == "" && len(schema.Properties) == 0 {
		// Unresolved reference; capture metadata for consumers to handle.
		field := Field{
			Name:        name,
			Type:        FieldTypeObject,
			Required:    required,
			Label:       b.opts.Labeler(name),
			Description: schema.Description,
			Metadata:    map[string]string{},
		}
		field.Metadata["$ref"] = schema.Ref
		refExt := metadataFromExtensions(schema.Extensions)
		mergeMetadata(field.Metadata, refExt)
		field.Relationship = relationshipFromExtensions(schema.Extensions)
		field.UIHints = mergeUIHints(field.UIHints, filterUIHints(refExt))
		applyRelationshipHints(&field)
		field.applyUIHintAttributes()
		field.normalizeMetadata()
		field.normalizeUIHints()
		return []Field{field}, nil
	}

	switch schema.Type {
	case "object", "":
		return b.fieldsFromObject(name, schema, required)
	case "array":
		field, err := b.fieldFromArray(name, schema, required)
		if err != nil {
			return nil, err
		}
		return []Field{field}, nil
	default:
		field := b.fieldFromPrimitive(name, schema, required)
		return []Field{field}, nil
	}
}

func (b *Builder) fieldsFromObject(name string, schema pkgopenapi.Schema, required bool) ([]Field, error) {
	var fields []Field
	requiredSet := make(map[string]struct{}, len(schema.Required))
	for _, item := range schema.Required {
		requiredSet[item] = struct{}{}
	}

	propNames := make([]string, 0, len(schema.Properties))
	for propName := range schema.Properties {
		propNames = append(propNames, propName)
	}
	sort.Strings(propNames)

	for _, propName := range propNames {
		propSchema := schema.Properties[propName]
		_, isRequired := requiredSet[propName]
		converted, err := b.fieldsFromSchema(propName, propSchema, isRequired)
		if err != nil {
			return nil, err
		}
		fields = append(fields, converted...)
	}

	decorateRelationshipSiblings(fields)

	if name != "" {
		// Wrap nested properties inside a parent object field.
		parent := Field{
			Name:        name,
			Type:        FieldTypeObject,
			Label:       b.opts.Labeler(name),
			Description: schema.Description,
			Required:    required,
			Nested:      fields,
		}
		if schema.Default != nil {
			parent.Default = schema.Default
		}
		if len(schema.Enum) > 0 {
			parent.Enum = append([]any(nil), schema.Enum...)
		}
		applyValidations(&parent, schema)
		parentExt := metadataFromExtensions(schema.Extensions)
		mergeMetadata(parent.ensureMetadata(), parentExt)
		parent.Relationship = relationshipFromExtensions(schema.Extensions)
		parent.UIHints = mergeUIHints(parent.UIHints, filterUIHints(parentExt))
		applyRelationshipHints(&parent)
		parent.applyUIHintAttributes()
		parent.normalizeMetadata()
		parent.normalizeUIHints()
		return []Field{parent}, nil
	}

	return fields, nil
}

func (b *Builder) fieldFromArray(name string, schema pkgopenapi.Schema, required bool) (Field, error) {
	if schema.Items == nil {
		return Field{}, fmt.Errorf("model builder: array field %q missing items", name)
	}
	var itemField *Field
	nested, err := b.fieldsFromSchema(name+"Item", *schema.Items, false)
	if err != nil {
		return Field{}, err
	}
	if len(nested) > 0 {
		item := nested[0]
		itemField = &item
	}

	field := Field{
		Name:        name,
		Type:        FieldTypeArray,
		Label:       b.opts.Labeler(name),
		Description: schema.Description,
		Required:    required,
		Items:       itemField,
	}
	if schema.Default != nil {
		field.Default = schema.Default
	}
	if len(schema.Enum) > 0 {
		field.Enum = append([]any(nil), schema.Enum...)
	}
	applyValidations(&field, schema)
	arrayExt := metadataFromExtensions(schema.Extensions)
	mergeMetadata(field.ensureMetadata(), arrayExt)
	field.Relationship = relationshipFromExtensions(schema.Extensions)
	field.UIHints = mergeUIHints(field.UIHints, filterUIHints(arrayExt))
	applyRelationshipHints(&field)
	propagateRelationshipToItems(&field)
	field.applyUIHintAttributes()
	field.normalizeMetadata()
	field.normalizeUIHints()
	return field, nil
}

func (b *Builder) fieldFromPrimitive(name string, schema pkgopenapi.Schema, required bool) Field {
	field := Field{
		Name:        name,
		Type:        mapType(schema.Type),
		Format:      schema.Format,
		Label:       b.opts.Labeler(name),
		Description: schema.Description,
		Required:    required,
		Default:     schema.Default,
	}
	if len(schema.Enum) > 0 {
		field.Enum = append([]any(nil), schema.Enum...)
	}
	if schema.Default != nil {
		field.Default = schema.Default
	}
	applyValidations(&field, schema)
	primitiveExt := metadataFromExtensions(schema.Extensions)
	mergeMetadata(field.ensureMetadata(), primitiveExt)
	field.Relationship = relationshipFromExtensions(schema.Extensions)
	field.UIHints = mergeUIHints(field.UIHints, filterUIHints(primitiveExt))
	applyFormatHints(&field)
	applyRelationshipHints(&field)
	field.applyUIHintAttributes()
	field.normalizeMetadata()
	field.normalizeUIHints()
	return field
}

func mapType(schemaType string) FieldType {
	switch schemaType {
	case "integer":
		return FieldTypeInteger
	case "number":
		return FieldTypeNumber
	case "boolean":
		return FieldTypeBoolean
	case "array":
		return FieldTypeArray
	case "object":
		return FieldTypeObject
	default:
		return FieldTypeString
	}
}

func applyValidations(field *Field, schema pkgopenapi.Schema) {
	if field == nil {
		return
	}

	if schema.Minimum != nil {
		params := map[string]string{
			"value": formatFloat(*schema.Minimum),
		}
		if schema.ExclusiveMinimum {
			params["exclusive"] = "true"
		}
		field.Validations = append(field.Validations, ValidationRule{
			Kind:   ValidationRuleMin,
			Params: params,
		})
	}

	if schema.Maximum != nil {
		params := map[string]string{
			"value": formatFloat(*schema.Maximum),
		}
		if schema.ExclusiveMaximum {
			params["exclusive"] = "true"
		}
		field.Validations = append(field.Validations, ValidationRule{
			Kind:   ValidationRuleMax,
			Params: params,
		})
	}

	if schema.MinLength != nil {
		field.Validations = append(field.Validations, ValidationRule{
			Kind: ValidationRuleMinLength,
			Params: map[string]string{
				"value": strconv.Itoa(*schema.MinLength),
			},
		})
	}

	if schema.MaxLength != nil {
		field.Validations = append(field.Validations, ValidationRule{
			Kind: ValidationRuleMaxLength,
			Params: map[string]string{
				"value": strconv.Itoa(*schema.MaxLength),
			},
		})
	}

	if schema.Pattern != "" {
		field.Validations = append(field.Validations, ValidationRule{
			Kind: ValidationRulePattern,
			Params: map[string]string{
				"pattern": schema.Pattern,
			},
		})
	}

	if len(field.Validations) == 0 {
		field.Validations = nil
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func metadataFromExtensions(ext map[string]any) map[string]string {
	if len(ext) == 0 {
		return nil
	}

	result := make(map[string]string)
	var labelField string

	for key, value := range ext {
		if key == extensionNamespace {
			nested, ok := value.(map[string]any)
			if !ok {
				continue
			}
			for nestedKey, nestedValue := range nested {
				if str, ok := CanonicalizeExtensionValue(nestedValue); ok {
					result[nestedKey] = str
					if nestedKey == "label-field" {
						labelField = str
					}
				}
			}
			continue
		}
		if strings.HasPrefix(key, extensionNamespace+"-") {
			trimmed := strings.TrimPrefix(key, extensionNamespace+"-")
			if str, ok := CanonicalizeExtensionValue(value); ok {
				result[trimmed] = str
				if trimmed == "label-field" {
					labelField = str
				}
			}
			continue
		}
	}

	if endpointMeta := endpointMetadataFromExtensions(ext); len(endpointMeta) > 0 {
		if len(result) == 0 {
			result = make(map[string]string, len(endpointMeta))
		}
		mergeMetadata(result, endpointMeta)
	}

	if currentValue, ok := currentValueFromExtensions(ext); ok {
		if len(result) == 0 {
			result = make(map[string]string, 1)
		}
		result["relationship.current"] = currentValue
	}

	if labelField != "" {
		if len(result) == 0 {
			result = make(map[string]string, 1)
		}
		result["relationship.endpoint.labelField"] = labelField
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func mergeMetadata(target map[string]string, updates map[string]string) {
	if len(updates) == 0 {
		return
	}
	if target == nil {
		return
	}

	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		target[key] = updates[key]
	}
}

func mergeUIHints(target map[string]string, updates map[string]string) map[string]string {
	if len(updates) == 0 {
		return target
	}
	if target == nil {
		target = make(map[string]string, len(updates))
	}
	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		target[key] = updates[key]
	}
	return target
}

func applyFormatHints(field *Field) {
	if field == nil {
		return
	}

	format := strings.TrimSpace(strings.ToLower(field.Format))
	if format == "" {
		return
	}

	if field.UIHints != nil {
		if current := strings.TrimSpace(field.UIHints["inputType"]); current != "" {
			return
		}
	}

	var inputType string
	switch format {
	case "date":
		inputType = "date"
	case "time":
		inputType = "time"
	case "date-time", "datetime", "datetime-local":
		inputType = "datetime-local"
	case "email":
		inputType = "email"
	case "uri", "iri", "uri-reference", "iri-reference", "url":
		inputType = "url"
	case "tel", "phone":
		inputType = "tel"
	case "password":
		inputType = "password"
	case "byte", "binary":
		inputType = "file"
	default:
		return
	}

	if field.UIHints == nil {
		field.UIHints = make(map[string]string, 1)
	}
	field.UIHints["inputType"] = inputType
}

func applyRelationshipHints(field *Field) {
	if field == nil || field.Relationship == nil {
		return
	}

	hints := make(map[string]string)
	switch strings.ToLower(string(field.Relationship.Kind)) {
	case "belongsto", "hasone":
		switch field.Type {
		case FieldTypeArray:
			hints["input"] = "collection"
		case FieldTypeObject:
			if shouldRenderObjectRelationshipAsSelect(field) {
				field.Type = FieldTypeString
				hints["input"] = "select"
			} else {
				hints["input"] = "subform"
			}
		default:
			hints["input"] = "select"
		}
	case "hasmany":
		switch field.Type {
		case FieldTypeArray:
			hints["input"] = "collection"
		case FieldTypeObject:
			if shouldRenderObjectRelationshipAsSelect(field) {
				field.Type = FieldTypeString
				hints["input"] = "select"
			} else {
				hints["input"] = "subform"
			}
		default:
			hints["input"] = "collection"
		}
	default:
		if field.Relationship.Cardinality == "many" {
			hints["input"] = "collection"
		} else if field.Type == FieldTypeArray {
			hints["input"] = "collection"
		} else if shouldRenderObjectRelationshipAsSelect(field) {
			field.Type = FieldTypeString
			hints["input"] = "select"
		} else {
			hints["input"] = "select"
		}
	}

	if card := field.Relationship.Cardinality; card != "" {
		hints["cardinality"] = card
	}
	if field.Type == FieldTypeArray && hasRelationshipEndpoint(field.Metadata) && hints["input"] == "collection" {
		hints["collectionRenderer"] = "chips"
		if metadata := field.ensureMetadata(); metadata["relationship.endpoint.renderer"] == "" {
			metadata["relationship.endpoint.renderer"] = "chips"
		}
	}

	if len(hints) == 0 {
		return
	}
	field.UIHints = mergeUIHints(field.UIHints, hints)
}

func shouldRenderObjectRelationshipAsSelect(field *Field) bool {
	if field == nil {
		return false
	}
	if field.Type != FieldTypeObject {
		return false
	}
	if field.Relationship != nil && field.Relationship.SourceField != "" {
		return false
	}
	if len(field.Nested) > 0 {
		return false
	}
	return hasRelationshipEndpoint(field.Metadata)
}

func hasRelationshipEndpoint(metadata map[string]string) bool {
	if len(metadata) == 0 {
		return false
	}
	for key := range metadata {
		if strings.HasPrefix(key, "relationship.endpoint.") {
			return true
		}
	}
	return false
}

func decorateRelationshipSiblings(fields []Field) {
	if len(fields) == 0 {
		return
	}
	index := make(map[string]int, len(fields))
	for i := range fields {
		index[fields[i].Name] = i
	}
	for i := range fields {
		field := &fields[i]
		if field.Relationship == nil || field.Relationship.SourceField == "" {
			continue
		}

		hostIdx, ok := index[field.Relationship.SourceField]
		if !ok {
			continue
		}
		host := &fields[hostIdx]
		if host.Relationship == nil {
			continue
		}
		cloned := cloneRelationship(host.Relationship)
		cloned.SourceField = field.Relationship.SourceField
		field.Relationship = cloned
		applyRelationshipHints(field)

		if field.Label != "" {
			host.Label = field.Label
		}
		if field.Description != "" && host.Description == "" {
			host.Description = field.Description
		}
		if len(field.UIHints) > 0 {
			if value, ok := field.UIHints["placeholder"]; ok && value != "" && host.Placeholder == "" {
				host.Placeholder = value
			}
			if value, ok := field.UIHints["label"]; ok && value != "" {
				host.Label = value
			}
			if value, ok := field.UIHints["hint"]; ok && value != "" && host.Description == "" {
				host.Description = value
			}
			if value, ok := field.UIHints["helpText"]; ok && value != "" {
				host.ensureMetadata()["helpText"] = value
			}
		}
	}
}

func propagateRelationshipToItems(field *Field) {
	if field == nil {
		return
	}
	applyRelationshipHints(field)
	if field.Items == nil {
		return
	}

	if field.Relationship != nil {
		cloned := cloneRelationship(field.Relationship)
		cloned.SourceField = ""
		field.Items.Relationship = cloned
		applyRelationshipHints(field.Items)
	}
}

func (f *Field) ensureMetadata() map[string]string {
	if f.Metadata == nil {
		f.Metadata = make(map[string]string)
	}
	return f.Metadata
}

func (f *Field) normalizeMetadata() {
	if f.Metadata != nil && len(f.Metadata) == 0 {
		f.Metadata = nil
	}
}

func (f *Field) normalizeUIHints() {
	if f.UIHints != nil && len(f.UIHints) == 0 {
		f.UIHints = nil
	}
}

func (f *Field) applyUIHintAttributes() {
	if len(f.UIHints) == 0 {
		return
	}
	if f.Placeholder == "" {
		if placeholder, ok := f.UIHints["placeholder"]; ok && placeholder != "" {
			f.Placeholder = placeholder
		}
	}
	if label, ok := f.UIHints["label"]; ok && label != "" {
		f.Label = label
	}
	if hint, ok := f.UIHints["hint"]; ok && hint != "" && f.Description == "" {
		f.Description = hint
	}
	if help, ok := f.UIHints["helpText"]; ok && help != "" {
		// Attach as an additional metadata entry so renderers without dedicated
		// UI hint support can still surface the string.
		if f.Metadata == nil {
			f.Metadata = make(map[string]string)
		}
		f.Metadata["helpText"] = help
	}
}

var fieldPlaceholderPattern = regexp.MustCompile(`\{\{\s*field:([^\}\s]+)\s*\}\}`)

func endpointMetadataFromExtensions(ext map[string]any) map[string]string {
	if len(ext) == 0 {
		return nil
	}
	raw, ok := ext[endpointExtensionKey]
	if !ok {
		return nil
	}

	endpointMap := toAnyMap(raw)
	if len(endpointMap) == 0 {
		return nil
	}

	meta := make(map[string]string)
	add := func(key, value string) {
		if value == "" {
			return
		}
		meta[key] = value
	}

	add("relationship.endpoint.url", strings.TrimSpace(toString(endpointMap["url"])))
	if method := strings.TrimSpace(toString(endpointMap["method"])); method != "" {
		add("relationship.endpoint.method", strings.ToUpper(method))
	}
	add("relationship.endpoint.labelField", strings.TrimSpace(toString(endpointMap["labelField"])))
	add("relationship.endpoint.valueField", strings.TrimSpace(toString(endpointMap["valueField"])))
	add("relationship.endpoint.resultsPath", strings.TrimSpace(toString(endpointMap["resultsPath"])))
	add("relationship.endpoint.mode", strings.TrimSpace(toString(endpointMap["mode"])))
	add("relationship.endpoint.searchParam", strings.TrimSpace(toString(endpointMap["searchParam"])))
	add("relationship.endpoint.submitAs", strings.TrimSpace(toString(endpointMap["submitAs"])))

	if params := toStringMap(endpointMap["params"]); len(params) > 0 {
		for _, key := range sortedKeys(params) {
			add("relationship.endpoint.params."+key, params[key])
		}
	}
	dynamicParams := toStringMap(endpointMap["dynamicParams"])
	if len(dynamicParams) > 0 {
		for _, key := range sortedKeys(dynamicParams) {
			add("relationship.endpoint.dynamicParams."+key, dynamicParams[key])
		}
		if refs := extractFieldReferences(dynamicParams); len(refs) > 0 {
			add("relationship.endpoint.refreshOn", strings.Join(refs, ","))
		}
	}

	if mapping := toStringMap(endpointMap["mapping"]); len(mapping) > 0 {
		if valuePath := strings.TrimSpace(mapping["value"]); valuePath != "" {
			add("relationship.endpoint.mapping.value", valuePath)
		}
		if labelPath := strings.TrimSpace(mapping["label"]); labelPath != "" {
			add("relationship.endpoint.mapping.label", labelPath)
		}
	}

	if auth := toStringMap(endpointMap["auth"]); len(auth) > 0 {
		add("relationship.endpoint.auth.strategy", strings.TrimSpace(auth["strategy"]))
		add("relationship.endpoint.auth.header", strings.TrimSpace(auth["header"]))
		add("relationship.endpoint.auth.source", strings.TrimSpace(auth["source"]))
	}

	if len(meta) == 0 {
		return nil
	}
	return meta
}

func currentValueFromExtensions(ext map[string]any) (string, bool) {
	if len(ext) == 0 {
		return "", false
	}
	value, ok := ext[currentValueExtensionKey]
	if !ok || value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", false
		}
		return v, true
	default:
		payload, err := json.Marshal(v)
		if err != nil || len(payload) == 0 {
			return "", false
		}
		return string(payload), true
	}
}

func toStringMap(value any) map[string]string {
	switch mapped := value.(type) {
	case map[string]string:
		return cloneStringMap(mapped)
	case map[string]any:
		out := make(map[string]string, len(mapped))
		for key, val := range mapped {
			str, ok := toStringValue(val)
			if !ok {
				continue
			}
			out[key] = str
		}
		return out
	default:
		return nil
	}
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

func toAnyMap(value any) map[string]any {
	switch mapped := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(mapped))
		for key, val := range mapped {
			cloned[key] = val
		}
		return cloned
	case map[string]string:
		cloned := make(map[string]any, len(mapped))
		for key, val := range mapped {
			cloned[key] = val
		}
		return cloned
	default:
		return nil
	}
}

func toStringValue(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", false
		}
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func toString(value any) string {
	str, ok := toStringValue(value)
	if !ok {
		return ""
	}
	return str
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func extractFieldReferences(dynamicParams map[string]string) []string {
	if len(dynamicParams) == 0 {
		return nil
	}
	refs := make(map[string]struct{})
	for _, value := range dynamicParams {
		for _, match := range fieldPlaceholderPattern.FindAllStringSubmatch(value, -1) {
			if len(match) < 2 {
				continue
			}
			name := strings.TrimSpace(match[1])
			if name == "" {
				continue
			}
			refs[name] = struct{}{}
		}
	}
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for name := range refs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func filterUIHints(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]string)
	for key, value := range metadata {
		if value == "" {
			continue
		}
		if IsAllowedUIHintKey(key) {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
