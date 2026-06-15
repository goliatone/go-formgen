package json

import (
	"context"
	stdjson "encoding/json"
	"fmt"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
)

const descriptorVersion = "formgen.descriptor/v1"

type Option func(*Renderer)

// WithoutEnvelope makes the renderer emit only the ordered form model.
func WithoutEnvelope() Option {
	return func(r *Renderer) {
		r.withoutEnvelope = true
	}
}

type Renderer struct {
	withoutEnvelope bool
}

func New(options ...Option) *Renderer {
	r := &Renderer{}
	for _, opt := range options {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *Renderer) Name() string {
	return "json"
}

func (r *Renderer) ContentType() string {
	return "application/json; charset=utf-8"
}

func (r *Renderer) Render(_ context.Context, form model.FormModel, options render.RenderOptions) ([]byte, error) {
	render.ApplySubset(&form, options.Subset)
	render.LocalizeFormModel(&form, options)
	render.RedactSensitiveDefaults(&form, options.IncludeSensitiveDefaults)

	if r != nil && r.withoutEnvelope {
		return marshal(form)
	}

	mapped := render.MapErrorPayload(form, options.Errors)
	envelope := Descriptor{
		Version:      descriptorVersion,
		Form:         form,
		Values:       render.RedactSensitiveValues(form, options.Values, options.IncludeSensitiveDefaults),
		Errors:       mapped.Fields,
		FormErrors:   render.MergeFormErrors(options.FormErrors, mapped.Form...),
		HiddenFields: render.SortedHiddenFields(options.HiddenFields),
		Metadata: map[string]any{
			"renderer":   "json",
			"renderMode": string(renderMode(options.RenderMode)),
		},
	}
	return marshal(envelope)
}

type Descriptor struct {
	Version      string               `json:"version"`
	Form         model.FormModel      `json:"form"`
	Values       map[string]any       `json:"values,omitempty"`
	Errors       map[string][]string  `json:"errors,omitempty"`
	FormErrors   []string             `json:"formErrors,omitempty"`
	HiddenFields []render.HiddenField `json:"hiddenFields,omitempty"`
	Metadata     map[string]any       `json:"metadata,omitempty"`
}

func marshal(value any) ([]byte, error) {
	payload, err := stdjson.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json renderer: marshal descriptor: %w", err)
	}
	payload = append(payload, '\n')
	return payload, nil
}

func renderMode(mode render.RenderMode) render.RenderMode {
	if mode == "" {
		return render.RenderModeDocument
	}
	return mode
}
