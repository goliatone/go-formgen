package model

import (
	"errors"
	"fmt"

	"github.com/goliatone/go-formgen/pkg/schema"
)

var (
	errOperationIDMissing     = errors.New("model builder: operation id is required")
	errOperationPathMissing   = errors.New("model builder: operation path is required")
	errOperationMethodMissing = errors.New("model builder: operation method is required")
)

func validateForm(form schema.Form) error {
	if form.ID == "" {
		return errOperationIDMissing
	}
	if form.Endpoint == "" {
		return errOperationPathMissing
	}
	if form.Method == "" {
		return errOperationMethodMissing
	}
	if err := validateSchema(form.Schema); err != nil {
		return fmt.Errorf("model builder: invalid request body: %w", err)
	}
	return nil
}

func validateSchema(schema schema.Schema) error {
	if schema.Type == "array" && schema.Items == nil {
		return errors.New("array schema requires items")
	}
	if schema.Type == "object" {
		for _, nested := range schema.Properties {
			if err := validateSchema(nested); err != nil {
				return err
			}
		}
	}
	if schema.Items != nil {
		return validateSchema(*schema.Items)
	}
	return nil
}
