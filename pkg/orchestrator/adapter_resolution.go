package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/schema"
)

func (o *Orchestrator) resolveAdapter(ctx context.Context, req Request) (schema.FormatAdapter, error) {
	if o.adapterRegistry == nil {
		return nil, errors.New("orchestrator: adapter registry is nil")
	}

	format := strings.TrimSpace(req.Format)
	if format != "" {
		adapter, err := o.adapterRegistry.Get(format)
		if err != nil {
			return nil, err
		}
		return adapter, nil
	}

	raw, src, err := o.rawForDetection(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		if o.defaultAdapter == "" {
			return nil, errors.New("orchestrator: format is required")
		}
		return o.adapterRegistry.Get(o.defaultAdapter)
	}

	matches := o.adapterRegistry.Detect(src, raw)
	switch len(matches) {
	case 0:
		if o.defaultAdapter == "" {
			return nil, errors.New("orchestrator: unable to detect format")
		}
		return o.adapterRegistry.Get(o.defaultAdapter)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("orchestrator: multiple adapters matched payload (%s), specify format", formatAdapterNames(matches))
	}
}

func (o *Orchestrator) resolveSchemaDocument(ctx context.Context, req Request, adapter schema.FormatAdapter) (schema.Document, error) {
	if req.SchemaDocument != nil {
		return *req.SchemaDocument, nil
	}
	if req.Document != nil {
		return schemaDocumentFromOpenAPI(*req.Document)
	}
	if req.Source == nil {
		return schema.Document{}, errors.New("orchestrator: source or document is required")
	}
	if adapter == nil {
		return schema.Document{}, errors.New("orchestrator: adapter is nil")
	}
	doc, err := adapter.Load(ctx, req.Source)
	if err != nil {
		return schema.Document{}, fmt.Errorf("orchestrator: load document: %w", err)
	}
	return doc, nil
}

func (o *Orchestrator) rawForDetection(ctx context.Context, req Request) ([]byte, schema.Source, error) {
	switch {
	case req.SchemaDocument != nil:
		return req.SchemaDocument.Raw(), req.SchemaDocument.Source(), nil
	case req.Document != nil:
		return req.Document.Raw(), req.Document.Source(), nil
	case req.Source != nil:
		raw, err := o.loadRaw(ctx, req.Source)
		if err != nil {
			return nil, nil, err
		}
		return raw, req.Source, nil
	default:
		return nil, nil, errors.New("orchestrator: source or document is required")
	}
}

func (o *Orchestrator) loadRaw(ctx context.Context, src pkgopenapi.Source) ([]byte, error) {
	if o.loader == nil && o.jsonSchemaLoader == nil {
		return nil, errors.New("orchestrator: loader is nil")
	}

	if o.loader != nil {
		doc, err := o.loader.Load(ctx, src)
		if err == nil {
			return doc.Raw(), nil
		}
		if o.jsonSchemaLoader == nil {
			return nil, fmt.Errorf("orchestrator: load document for detection: %w", err)
		}
	}

	if o.jsonSchemaLoader != nil {
		doc, err := o.jsonSchemaLoader.Load(ctx, src)
		if err != nil {
			return nil, fmt.Errorf("orchestrator: load document for detection: %w", err)
		}
		return doc.Raw(), nil
	}

	return nil, errors.New("orchestrator: loader is nil")
}

func schemaDocumentFromOpenAPI(doc pkgopenapi.Document) (schema.Document, error) {
	src := doc.Source()
	raw := doc.Raw()
	if src == nil && len(raw) == 0 {
		return schema.Document{}, nil
	}
	return schema.NewDocument(src, raw)
}

func formatFormRefs(refs []schema.FormRef) string {
	if len(refs) == 0 {
		return "none"
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		ids = append(ids, ref.ID)
	}
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

func formatAdapterNames(adapters []schema.FormatAdapter) string {
	if len(adapters) == 0 {
		return ""
	}
	names := make([]string, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		name := strings.TrimSpace(adapter.Name())
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ", ")
}
