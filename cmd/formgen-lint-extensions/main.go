package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goliatone/formgen"
	internalmodel "github.com/goliatone/formgen/internal/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

const extensionNamespace = "x-formgen"

type violation struct {
	file     string
	location string
	message  string
}

func main() {
	flag.Usage = func() {
		if _, err := fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [paths...]\n", filepath.Base(os.Args[0])); err != nil {
			panic(err)
		}
		if _, err := fmt.Fprintf(flag.CommandLine.Output(), "\nLint OpenAPI documents for unsupported formgen UI extensions.\n"); err != nil {
			panic(err)
		}
	}
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{
			"examples/fixtures/petstore.json",
			"examples/http/schema.json",
		}
	}

	ctx := context.Background()
	parser := formgen.NewParser(
		pkgopenapi.WithPartialDocuments(true),
		pkgopenapi.WithReferenceResolution(false),
	)

	var (
		violations []violation
	)
	for _, path := range paths {
		linted, err := lintFile(ctx, parser, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lint %s: %v\n", path, err)
			os.Exit(1)
		}
		violations = append(violations, linted...)
	}

	if len(violations) > 0 {
		sort.Slice(violations, func(i, j int) bool {
			if violations[i].file == violations[j].file {
				if violations[i].location == violations[j].location {
					return violations[i].message < violations[j].message
				}
				return violations[i].location < violations[j].location
			}
			return violations[i].file < violations[j].file
		})
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "%s: %s -> %s\n", v.file, v.location, v.message)
		}
		os.Exit(1)
	}
}

func lintFile(ctx context.Context, parser pkgopenapi.Parser, path string) ([]violation, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	doc, err := pkgopenapi.NewDocument(pkgopenapi.SourceFromFile(path), raw)
	if err != nil {
		return nil, fmt.Errorf("construct document: %w", err)
	}

	operations, err := parser.Operations(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("parse operations: %w", err)
	}

	var result []violation
	for id, op := range operations {
		base := []string{"operation", id}
		result = append(result, lintExtensions(path, base, op.Extensions)...)
		if len(op.RequestBody.Extensions) > 0 {
			result = append(result, lintExtensions(path, append(base, "requestBody"), op.RequestBody.Extensions)...)
		}
		result = append(result, lintSchema(path, append(base, "requestBody"), op.RequestBody)...)

		codes := make([]string, 0, len(op.Responses))
		for code := range op.Responses {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			response := op.Responses[code]
			result = append(result, lintExtensions(path, append(base, "responses", code), response.Extensions)...)
			result = append(result, lintSchema(path, append(base, "responses", code), response)...)
		}
	}

	return result, nil
}

func lintSchema(file string, path []string, schema pkgopenapi.Schema) []violation {
	var result []violation
	if len(schema.Extensions) > 0 {
		result = append(result, lintExtensions(file, path, schema.Extensions)...)
	}

	if len(schema.Properties) > 0 {
		keys := make([]string, 0, len(schema.Properties))
		for key := range schema.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := appendPath(path, "properties."+key)
			result = append(result, lintSchema(file, next, schema.Properties[key])...)
		}
	}

	if schema.Items != nil {
		result = append(result, lintSchema(file, appendPath(path, "items"), *schema.Items)...)
	}

	return result
}

func lintExtensions(file string, path []string, extensions map[string]any) []violation {
	if len(extensions) == 0 {
		return nil
	}

	var result []violation
	sortedKeys := make([]string, 0, len(extensions))
	for key := range extensions {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		value := extensions[key]
		switch {
		case key == extensionNamespace:
			nested, ok := value.(map[string]any)
			if !ok {
				result = append(result, violation{
					file:     file,
					location: formatLocation(path),
					message:  fmt.Sprintf("%s must be an object, found %T", extensionNamespace, value),
				})
				continue
			}
			nestedKeys := make([]string, 0, len(nested))
			for nestedKey := range nested {
				nestedKeys = append(nestedKeys, nestedKey)
			}
			sort.Strings(nestedKeys)
			for _, nestedKey := range nestedKeys {
				result = append(result, validateHint(file, appendPath(path, nestedKey), nestedKey, nested[nestedKey])...)
			}
		case strings.HasPrefix(key, extensionNamespace+"-"):
			trimmed := strings.TrimPrefix(key, extensionNamespace+"-")
			result = append(result, validateHint(file, path, trimmed, value)...)
		}
	}

	return result
}

func validateHint(file string, path []string, key string, value any) []violation {
	var result []violation
	if key == "" {
		result = append(result, violation{
			file:     file,
			location: formatLocation(path),
			message:  "extension key is empty",
		})
		return result
	}

	if !internalmodel.IsAllowedUIHintKey(key) {
		result = append(result, violation{
			file:     file,
			location: formatLocation(path),
			message:  fmt.Sprintf("unsupported UI extension key %q (supported: %s)", key, strings.Join(internalmodel.AllowedUIHintKeys(), ", ")),
		})
		return result
	}

	if _, ok := internalmodel.CanonicalizeExtensionValue(value); !ok {
		result = append(result, violation{
			file:     file,
			location: formatLocation(path),
			message:  fmt.Sprintf("value for %q must be a string, number, or boolean (got %T)", key, value),
		})
	}

	return result
}

func appendPath(path []string, segment string) []string {
	next := append([]string(nil), path...)
	next = append(next, segment)
	return next
}

func formatLocation(path []string) string {
	return strings.Join(path, " > ")
}
