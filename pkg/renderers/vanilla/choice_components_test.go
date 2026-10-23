package vanilla_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/submission"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

func renderChoiceForm(t *testing.T, fields ...model.Field) string {
	t.Helper()
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	form := model.FormModel{OperationID: "choices", Endpoint: "/choices", Method: "POST", Fields: fields}
	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{RenderMode: render.RenderModeFields})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return string(output)
}

func countOccurrences(html, needle string) int {
	return strings.Count(html, needle)
}

func TestChoiceComponentRendersUnansweredTypedBooleanGroup(t *testing.T) {
	html := renderChoiceForm(t, model.Field{
		Name:        "enabled",
		Type:        model.FieldTypeBoolean,
		Label:       "Enable feature",
		Description: "Pick one",
		Required:    true,
		Metadata:    map[string]string{"component.name": "choice"},
	})
	for _, marker := range []string{
		`<fieldset id="fg-enabled"`,
		`data-formgen-choice="single"`,
		`role="radiogroup"`,
		`aria-labelledby="fg-enabled-legend"`,
		`aria-describedby="fg-enabled-description"`,
		`<legend id="fg-enabled-legend"`,
		`id="fg-enabled-description"`,
		`id="fg-enabled-0"`,
		`id="fg-enabled-1"`,
		`for="fg-enabled-0"`,
		`type="radio"`,
		`name="enabled"`,
		`value="` + submission.EncodeEnumValue(true) + `"`,
		`value="` + submission.EncodeEnumValue(false) + `"`,
		`<span class="formgen-choice-text">Yes</span>`,
		`<span class="formgen-choice-text">No</span>`,
		`data-relationship-error="true"`,
		`aria-required="true"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("missing %q in:\n%s", marker, html)
		}
	}
	if strings.Contains(html, " checked") {
		t.Fatalf("choice must start unanswered:\n%s", html)
	}
	if strings.Contains(html, `data-formgen-chrome="label"`) {
		t.Fatalf("wrapper label chrome must not duplicate the legend:\n%s", html)
	}
	if countOccurrences(html, " required") != 2 {
		t.Fatalf("every radio in a required group carries required:\n%s", html)
	}
	if countOccurrences(html, "aria-required") != 1 {
		t.Fatalf("aria-required belongs on the radiogroup fieldset, never on radio members:\n%s", html)
	}

	// The encoded radio value decodes to a typed boolean on the server.
	parsed := submission.ParseMap(model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}}}, map[string]any{"enabled": submission.EncodeEnumValue(false)})
	if len(parsed.Issues) != 0 || parsed.Values["enabled"] != false {
		t.Fatalf("typed false lost: %+v", parsed)
	}
}

func TestChoiceComponentHonorsDefaultsStatesAndCustomLabels(t *testing.T) {
	html := renderChoiceForm(t, model.Field{
		Name:     "mode",
		Type:     model.FieldTypeString,
		Label:    "Mode",
		Default:  "false",
		Enum:     []any{"false", "true"},
		Readonly: true,
		Metadata: map[string]string{"component.name": "choice", "validation.state": "invalid", "validation.message": "Pick a mode"},
	})
	if !strings.Contains(html, `value="false" class="formgen-choice-input" data-formgen-choice-index="0" checked disabled aria-readonly="true"`) {
		t.Fatalf("string false default, readonly state, or plain string value missing:\n%s", html)
	}
	if !strings.Contains(html, `aria-invalid="true" data-validation-state="invalid"`) || !strings.Contains(html, `>Pick a mode</p>`) {
		t.Fatalf("group error state missing:\n%s", html)
	}
	if !strings.Contains(html, `aria-describedby="fg-mode-error"`) {
		t.Fatalf("error slot must be referenced by the group:\n%s", html)
	}

	custom := renderChoiceForm(t, model.Field{
		Name:     "consent",
		Type:     model.FieldTypeBoolean,
		Label:    "Consent",
		Disabled: true,
		Metadata: map[string]string{"component.name": "choice", "component.config": `{"trueLabel":"Agree","falseLabel":"Decline"}`},
	})
	for _, marker := range []string{`>Agree</span>`, `>Decline</span>`, `disabled aria-disabled="true"`} {
		if !strings.Contains(custom, marker) {
			t.Fatalf("missing %q in:\n%s", marker, custom)
		}
	}
}

func TestChoiceMultipleRendersCheckboxGroupWithTypedItems(t *testing.T) {
	html := renderChoiceForm(t, model.Field{
		Name:     "colors",
		Type:     model.FieldTypeArray,
		Label:    "Colors",
		Required: true,
		Default:  []any{"green"},
		Items:    &model.Field{Type: model.FieldTypeString, Enum: []any{"blue", "green"}},
		Validations: []model.ValidationRule{
			{Kind: model.ValidationRuleMinItems, Params: map[string]string{"value": "1"}},
			{Kind: model.ValidationRuleUniqueItems},
		},
		Metadata: map[string]string{"component.name": "choice-multiple"},
	})
	for _, marker := range []string{
		`data-formgen-choice="multiple"`,
		`type="checkbox"`,
		`name="colors[]"`,
		`data-field-path="colors"`,
		`value="blue"`,
		`value="green" class="formgen-choice-input" data-formgen-choice-index="1" checked`,
		`data-formgen-required="true"`,
		`<span class="sr-only" data-formgen-required-hint="true">(required)</span>`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("missing %q in:\n%s", marker, html)
		}
	}
	if strings.Contains(html, `role="radiogroup"`) || strings.Contains(html, `aria-required`) || countOccurrences(html, " required") > 0 {
		t.Fatalf("checkbox groups must not use radiogroup/aria-required or native required members:\n%s", html)
	}

	// Distinct choices are enforced by the model rule, never inferred from markup.
	form := model.FormModel{Fields: []model.Field{{Name: "colors", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeString, Enum: []any{"blue", "green"}}, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}}}}
	parsed := submission.ParseMap(form, map[string]any{"colors": []any{"blue", "blue"}})
	issues := submission.Validate(form, parsed.Values)
	if len(issues) != 1 || issues[0].Code != submission.CodeUniqueItems {
		t.Fatalf("server-side distinct policy: %+v", issues)
	}

	typed := renderChoiceForm(t, model.Field{
		Name:     "answers",
		Type:     model.FieldTypeArray,
		Label:    "Answers",
		Items:    &model.Field{Type: model.FieldTypeBoolean, Enum: []any{false, true}},
		Metadata: map[string]string{"component.name": "choice-multiple"},
	})
	if !strings.Contains(typed, `value="`+submission.EncodeEnumValue(false)+`"`) {
		t.Fatalf("typed false item must use the encoded codec:\n%s", typed)
	}
}

func TestScaleComponentDerivesBoundsAndKeepsZero(t *testing.T) {
	oneToTen := renderChoiceForm(t, model.Field{
		Name:  "level",
		Type:  model.FieldTypeInteger,
		Label: "Level",
		Validations: []model.ValidationRule{
			{Kind: model.ValidationRuleMin, Params: map[string]string{"value": "1"}},
			{Kind: model.ValidationRuleMax, Params: map[string]string{"value": "10"}},
		},
		Metadata: map[string]string{"component.name": "scale", "component.config": `{"lowLabel":"Low","highLabel":"High"}`},
	})
	if got := countOccurrences(oneToTen, `type="radio"`); got != 10 {
		t.Fatalf("1-10 scale rendered %d radios:\n%s", got, oneToTen)
	}
	for _, marker := range []string{
		`data-formgen-choice="scale"`,
		`value="` + submission.EncodeEnumValue(int64(1)) + `"`,
		`value="` + submission.EncodeEnumValue(int64(10)) + `"`,
		`<span class="formgen-choice-text">10</span>`,
		`data-formgen-scale-end="low">Low</span>`,
		`data-formgen-scale-end="high">High</span>`,
	} {
		if !strings.Contains(oneToTen, marker) {
			t.Fatalf("missing %q in:\n%s", marker, oneToTen)
		}
	}
	if strings.Contains(oneToTen, " checked") {
		t.Fatalf("scale must start unanswered:\n%s", oneToTen)
	}

	zeroToTen := renderChoiceForm(t, model.Field{
		Name:    "score",
		Type:    model.FieldTypeInteger,
		Label:   "Score",
		Default: int64(0),
		Validations: []model.ValidationRule{
			{Kind: model.ValidationRuleMin, Params: map[string]string{"value": "0"}},
			{Kind: model.ValidationRuleMax, Params: map[string]string{"value": "10"}},
		},
		Metadata: map[string]string{"component.name": "scale"},
	})
	if got := countOccurrences(zeroToTen, `type="radio"`); got != 11 {
		t.Fatalf("0-10 scale rendered %d radios:\n%s", got, zeroToTen)
	}
	zeroValue := submission.EncodeEnumValue(int64(0))
	if !strings.Contains(zeroToTen, `value="`+zeroValue+`" class="formgen-choice-input" data-formgen-choice-index="0" checked`) {
		t.Fatalf("zero default must select the zero option:\n%s", zeroToTen)
	}
	parsed := submission.ParseMap(model.FormModel{Fields: []model.Field{{Name: "score", Type: model.FieldTypeInteger}}}, map[string]any{"score": zeroValue})
	if len(parsed.Issues) != 0 || parsed.Values["score"] != int64(0) {
		t.Fatalf("typed zero lost: %+v", parsed)
	}

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	unbounded := model.FormModel{Fields: []model.Field{{Name: "level", Type: model.FieldTypeInteger, Metadata: map[string]string{"component.name": "scale"}}}}
	if _, err := renderer.Render(testsupport.Context(), unbounded, render.RenderOptions{}); err == nil || !strings.Contains(err.Error(), "min and max") {
		t.Fatalf("scale without bounds must fail explicitly, got %v", err)
	}
	wrongType := model.FormModel{Fields: []model.Field{{Name: "name", Type: model.FieldTypeString, Metadata: map[string]string{"component.name": "scale"}}}}
	if _, err := renderer.Render(testsupport.Context(), wrongType, render.RenderOptions{}); err == nil {
		t.Fatal("scale on a string field must fail")
	}
	for _, bounds := range [][2]string{{"-5000000000000000000", "5000000000000000000"}, {"-9223372036854775808", "9223372036854775807"}, {"0", "1000"}} {
		extreme := model.FormModel{Fields: []model.Field{{Name: "huge", Type: model.FieldTypeInteger, Validations: []model.ValidationRule{{Kind: model.ValidationRuleMin, Params: map[string]string{"value": bounds[0]}}, {Kind: model.ValidationRuleMax, Params: map[string]string{"value": bounds[1]}}}, Metadata: map[string]string{"component.name": "scale"}}}}
		diagnostics := renderer.Preflight(extreme)
		if _, err := renderer.Render(testsupport.Context(), extreme, render.RenderOptions{}); err == nil || len(diagnostics) != 1 {
			t.Fatalf("bounds %v must be diagnosed, not rendered: err=%v diagnostics=%v", bounds, err, diagnostics)
		}
	}
	tight := model.FormModel{Fields: []model.Field{{Name: "edge", Type: model.FieldTypeInteger, Validations: []model.ValidationRule{{Kind: model.ValidationRuleMin, Params: map[string]string{"value": "9223372036854775802"}}, {Kind: model.ValidationRuleMax, Params: map[string]string{"value": "9223372036854775807"}}}, Metadata: map[string]string{"component.name": "scale"}}}}
	tightOutput, tightErr := renderer.Render(testsupport.Context(), tight, render.RenderOptions{RenderMode: render.RenderModeFields})
	if tightErr != nil || countOccurrences(string(tightOutput), `type="radio"`) != 6 {
		t.Fatalf("bounds near MaxInt64 must render six options without wrapping: err=%v", tightErr)
	}
}

func TestChoiceComponentsKeepUniqueIDsAcrossInstancesAndRows(t *testing.T) {
	first := renderChoiceForm(t, model.Field{Name: "enabled", Type: model.FieldTypeBoolean, Label: "Enabled", Metadata: map[string]string{"component.name": "choice", "control.id": "formA-enabled"}})
	second := renderChoiceForm(t, model.Field{Name: "enabled", Type: model.FieldTypeBoolean, Label: "Enabled", Metadata: map[string]string{"component.name": "choice", "control.id": "formB-enabled"}})
	if !strings.Contains(first, `id="formA-enabled-0"`) || !strings.Contains(second, `id="formB-enabled-0"`) || strings.Contains(second, "formA-") {
		t.Fatalf("instance ids must follow control.id:\n%s\n%s", first, second)
	}

	rows := renderChoiceForm(t, model.Field{
		Name:  "rows",
		Type:  model.FieldTypeArray,
		Label: "Rows",
		Default: []any{
			map[string]any{"answer": true},
			map[string]any{"answer": false},
		},
		Items: &model.Field{Type: model.FieldTypeObject, Nested: []model.Field{{Name: "answer", Type: model.FieldTypeBoolean, Label: "Answer", Metadata: map[string]string{"component.name": "choice"}}}},
	})
	for _, marker := range []string{`id="fg-rows-0-answer-0"`, `id="fg-rows-1-answer-0"`, `name="rows[0].answer"`, `name="rows[1].answer"`} {
		if !strings.Contains(rows, marker) {
			t.Fatalf("missing %q in:\n%s", marker, rows)
		}
	}
	if countOccurrences(rows, ` checked`) != 2 {
		t.Fatalf("each row keeps its own typed selection:\n%s", rows)
	}
}

func TestChoiceComponentsDoNotReplaceDefaultControls(t *testing.T) {
	html := renderChoiceForm(t,
		model.Field{Name: "flag", Type: model.FieldTypeBoolean, Label: "Flag"},
		model.Field{Name: "color", Type: model.FieldTypeString, Label: "Color", Enum: []any{"blue", "green"}},
		model.Field{Name: "tags", Type: model.FieldTypeArray, Label: "Tags", Items: &model.Field{Type: model.FieldTypeString, Enum: []any{"a", "b"}}},
	)
	if !strings.Contains(html, `type="checkbox"`) || !strings.Contains(html, `<select`) || strings.Contains(html, `data-formgen-choice`) {
		t.Fatalf("defaults must stay checkbox/select/repeater:\n%s", html)
	}
}

func TestSyntheticAuthoringChoicesRenderWithoutDefaults(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/embedded/authoring.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Form model.FormModel }
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	html := renderChoiceForm(t, fixture.Form.Fields...)
	for _, marker := range []string{`name="q_enabled"`, `data-formgen-choice="single"`, `data-formgen-choice="multiple"`, `name="q_scale"`, `readonly`, `value="` + submission.EncodeEnumValue(float64(0)) + `"`} {
		if !strings.Contains(html, marker) {
			t.Fatalf("missing %s", marker)
		}
	}
	if strings.Contains(html, " checked") {
		t.Fatal("synthetic choices must initially be unanswered")
	}
}
