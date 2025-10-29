package model

import (
	"errors"
	"fmt"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

var (
	errOperationIDMissing     = errors.New("model builder: operation id is required")
	errOperationPathMissing   = errors.New("model builder: operation path is required")
	errOperationMethodMissing = errors.New("model builder: operation method is required")
)

func validateOperation(op pkgopenapi.Operation) error {
	if op.ID == "" {
		return errOperationIDMissing
	}
	if op.Path == "" {
		return errOperationPathMissing
	}
	if op.Method == "" {
		return errOperationMethodMissing
	}
	if err := validateSchema(op.RequestBody); err != nil {
		return fmt.Errorf("model builder: invalid request body: %w", err)
	}
	return nil
}

func validateSchema(schema pkgopenapi.Schema) error {
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
