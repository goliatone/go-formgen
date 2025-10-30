package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
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

	fields, err := b.fieldsFromSchema("", op.RequestBody, true)
	if err != nil {
		return FormModel{}, err
	}
	form.Fields = fields

	if len(form.Metadata) == 0 {
		form.Metadata = nil
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
			Metadata: map[string]string{
				"$ref": schema.Ref,
			},
		}
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
