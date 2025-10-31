package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

const (
	extensionNamespace        = "x-formgen"
	relationshipNamespace     = "relationship."
	relationshipTypeKey       = relationshipNamespace + "type"
	relationshipTargetKey     = relationshipNamespace + "target"
	relationshipForeignKeyKey = relationshipNamespace + "foreignKey"
	relationshipCardKey       = relationshipNamespace + "cardinality"
	relationshipInverseKey    = relationshipNamespace + "inverse"
	relationshipSourceKey     = relationshipNamespace + "sourceField"
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
		ensureRelationship(&field)
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
		ensureRelationship(&parent)
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
	ensureRelationship(&field)
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
	ensureRelationship(&field)
	field.UIHints = mergeUIHints(field.UIHints, filterUIHints(primitiveExt))
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
	for key, value := range ext {
		if key == extensionNamespace {
			nested, ok := value.(map[string]any)
			if !ok {
				continue
			}
			for nestedKey, nestedValue := range nested {
				if str, ok := CanonicalizeExtensionValue(nestedValue); ok {
					result[nestedKey] = str
				}
			}
			continue
		}
		if strings.HasPrefix(key, extensionNamespace+"-") {
			trimmed := strings.TrimPrefix(key, extensionNamespace+"-")
			if str, ok := CanonicalizeExtensionValue(value); ok {
				result[trimmed] = str
			}
			continue
		}
		if strings.HasPrefix(key, relationshipNamespace) {
			if str, ok := CanonicalizeExtensionValue(value); ok {
				result[key] = str
			}
		}
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

func applyRelationshipHints(field *Field) {
	if field == nil {
		return
	}
	if len(field.Metadata) == 0 {
		return
	}
	relType, ok := field.Metadata[relationshipTypeKey]
	if !ok || relType == "" {
		return
	}

	hints := make(map[string]string)
	switch strings.ToLower(relType) {
	case "belongsto", "hasone":
		switch field.Type {
		case FieldTypeArray:
			hints["input"] = "collection"
		case FieldTypeObject:
			hints["input"] = "subform"
		default:
			hints["input"] = "select"
		}
	case "hasmany":
		switch field.Type {
		case FieldTypeArray:
			hints["input"] = "collection"
		case FieldTypeObject:
			hints["input"] = "subform"
		default:
			hints["input"] = "collection"
		}
	default:
		if field.Type == FieldTypeArray {
			hints["input"] = "collection"
		} else {
			hints["input"] = "select"
		}
	}

	if card := field.Metadata[relationshipCardKey]; card != "" {
		hints["cardinality"] = card
	}

	if len(hints) == 0 {
		return
	}
	field.UIHints = mergeUIHints(field.UIHints, hints)
}

func decorateRelationshipSiblings(fields []Field) {
	if len(fields) == 0 {
		return
	}
	index := make(map[string]int, len(fields))
	for i := range fields {
		index[fields[i].Name] = i
		ensureRelationship(&fields[i])
	}
	for i := range fields {
		field := &fields[i]
		if len(field.Metadata) == 0 {
			continue
		}
		if hostName := field.Metadata[relationshipSourceKey]; hostName != "" {
			if idx, ok := index[hostName]; ok {
				host := &fields[idx]
				ensureRelationship(host)
				copyRelationshipAttributes(field, host)
			}
		}
		ensureRelationship(field)
	}
}

func copyRelationshipAttributes(target, host *Field) {
	if target == nil || host == nil {
		return
	}
	if len(host.Metadata) != 0 {
		meta := target.ensureMetadata()
		for key, value := range host.Metadata {
			if !strings.HasPrefix(key, relationshipNamespace) {
				continue
			}
			if key == relationshipSourceKey {
				continue
			}
			meta[key] = value
		}
	}
	sourceField := ""
	if target.Metadata != nil && target.Metadata[relationshipSourceKey] != "" {
		sourceField = target.Metadata[relationshipSourceKey]
	}
	if target.Relationship != nil && target.Relationship.SourceField != "" {
		sourceField = target.Relationship.SourceField
	}

	if host.Relationship != nil {
		cloned := cloneRelationship(host.Relationship)
		cloned.SourceField = strings.TrimSpace(sourceField)
		target.Relationship = cloned
		target.Metadata = syncRelationshipMetadata(target.Metadata, cloned)
	} else {
		ensureRelationship(target)
	}

	applyRelationshipHints(target)
}

func propagateRelationshipToItems(field *Field) {
	if field == nil {
		return
	}
	applyRelationshipHints(field)
	if field.Items == nil {
		return
	}
	if len(field.Metadata) != 0 {
		meta := field.Items.ensureMetadata()
		for key, value := range field.Metadata {
			if !strings.HasPrefix(key, relationshipNamespace) {
				continue
			}
			if key == relationshipSourceKey {
				continue
			}
			meta[key] = value
		}
	}

	if field.Relationship != nil {
		cloned := cloneRelationship(field.Relationship)
		cloned.SourceField = ""
		field.Items.Relationship = cloned
		field.Items.Metadata = syncRelationshipMetadata(field.Items.Metadata, cloned)
	} else {
		ensureRelationship(field.Items)
	}

	applyRelationshipHints(field.Items)
	ensureRelationship(field.Items)
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
