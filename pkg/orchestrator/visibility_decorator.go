package orchestrator

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/visibility"
)

func applyVisibility(form *model.FormModel, evaluator visibility.Evaluator, ctx visibility.Context) error {
	if form == nil || evaluator == nil {
		return nil
	}

	fields, err := filterVisibleFields(form.Fields, "", evaluator, ctx)
	if err != nil {
		return fmt.Errorf("orchestrator: apply visibility: %w", err)
	}
	form.Fields = fields
	return nil
}

func filterVisibleFields(fields []model.Field, prefix string, evaluator visibility.Evaluator, ctx visibility.Context) ([]model.Field, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	result := make([]model.Field, 0, len(fields))
	for _, field := range fields {
		path := field.Name
		if prefix != "" {
			path = prefix + "." + field.Name
		}

		rule := visibilityRule(field)
		if rule != "" {
			ok, err := evaluator.Eval(path, rule, ctx)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}

		nested, err := filterVisibleFields(field.Nested, path, evaluator, ctx)
		if err != nil {
			return nil, err
		}
		field.Nested = nested

		if field.Items != nil {
			itemPath := path
			if itemPath != "" {
				itemPath += "[]"
			}
			item, keep, err := filterVisibleField(*field.Items, itemPath, evaluator, ctx)
			if err != nil {
				return nil, err
			}
			if keep {
				field.Items = &item
			} else {
				field.Items = nil
			}
		}

		result = append(result, field)
	}

	return result, nil
}

func filterVisibleField(field model.Field, path string, evaluator visibility.Evaluator, ctx visibility.Context) (model.Field, bool, error) {
	rule := visibilityRule(field)
	if rule != "" {
		ok, err := evaluator.Eval(path, rule, ctx)
		if err != nil {
			return field, false, err
		}
		if !ok {
			return field, false, nil
		}
	}

	nested, err := filterVisibleFields(field.Nested, path, evaluator, ctx)
	if err != nil {
		return field, false, err
	}
	field.Nested = nested

	if field.Items != nil {
		itemPath := path
		if itemPath != "" {
			itemPath += "[]"
		}
		item, keep, err := filterVisibleField(*field.Items, itemPath, evaluator, ctx)
		if err != nil {
			return field, false, err
		}
		if keep {
			field.Items = &item
		} else {
			field.Items = nil
		}
	}

	return field, true, nil
}

func visibilityRule(field model.Field) string {
	candidates := []string{
		field.Metadata["visibilityRule"],
		field.Metadata["admin.visibilityRule"],
		field.UIHints["visibilityRule"],
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

func visibilityContext(options render.RenderOptions) visibility.Context {
	ctx := options.VisibilityContext
	if ctx.Values == nil && len(options.Values) > 0 {
		ctx.Values = options.Values
	}
	return ctx
}
