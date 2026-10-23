package submission

import (
	"encoding/json"
	"math"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestFiniteNumbersRequiredByParsingAndValidation(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "n", Type: model.FieldTypeNumber, Required: true,
		Validations: []model.ValidationRule{{Kind: "min", Params: map[string]string{"value": "0"}}, {Kind: "max", Params: map[string]string{"value": "10"}}}}}}
	for _, input := range []any{"NaN", "Inf", "-Infinity", math.NaN(), math.Inf(1), math.Inf(-1), float32(math.NaN()), json.Number("NaN"), json.Number("1e9999")} {
		got := ParseMap(form, map[string]any{"n": input})
		if got.Values["n"] != nil || len(got.Issues) != 1 || got.Issues[0].Code != CodeType || got.Issues[0].Path != "n" {
			t.Fatalf("input=%v result=%+v", input, got)
		}
		if _, err := json.Marshal(got.Values); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []any{math.NaN(), math.Inf(1), math.Inf(-1), json.Number("NaN"), json.Number("1e9999")} {
		issues := Validate(form, Values{"n": input})
		if len(issues) != 1 || issues[0].Code != CodeType {
			t.Fatalf("direct validation accepted %v: %+v", input, issues)
		}
	}
	for _, text := range []string{"NaN", "Inf", "-Inf"} {
		if got := ParseValues(form, url.Values{"n": {text}}); len(got.Issues) != 1 || got.Issues[0].Code != CodeType {
			t.Fatal(got)
		}
	}
	for _, input := range []any{0, int64(10), uint64(5), 1.25, float32(2.5), "3e0", json.Number("4.5")} {
		got := ParseMap(form, map[string]any{"n": input})
		if len(got.Issues) != 0 || len(Validate(form, got.Values)) != 0 {
			t.Fatalf("finite input=%v: %+v", input, got)
		}
	}
}

func TestJSONIntegersPreserveExactDecimalValues(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "n", Type: model.FieldTypeInteger}}}
	valid := map[string]int64{
		"9223372036854775807": math.MaxInt64, "9223372036854775807.0": math.MaxInt64,
		"-9223372036854775808.0": math.MinInt64, "9.223372036854775807e18": math.MaxInt64,
		"9007199254740993.0": 9007199254740993, "1.20e1": 12, "100e-2": 1,
		"-0e99999999999999999999999999": 0,
	}
	for text, want := range valid {
		got, err := ParseJSON(form, []byte(`{"n":`+text+`}`))
		if err != nil || len(got.Issues) != 0 || got.Values["n"] != want {
			t.Fatalf("%s: %+v err=%v want=%v", text, got, err, want)
		}
	}
	for _, text := range []string{"9223372036854775808", "-9223372036854775809", "9223372036854775807.1", "1.0000000000000001", "0.99999999999999999", "1e2147483648", "1e-2147483648", "1e" + strings.Repeat("9", 2000)} {
		got, err := ParseJSON(form, []byte(`{"n":`+text+`}`))
		if err != nil || got.Values["n"] != nil || len(got.Issues) != 1 || got.Issues[0].Code != CodeType {
			t.Fatalf("%s: %+v err=%v", text[:min(len(text), 40)], got, err)
		}
	}
	for _, input := range []any{float64(0x1p63), float32(0x1p63), math.Nextafter(-0x1p63, math.Inf(-1)), json.Number("01"), json.Number("+1"), json.Number("1 2")} {
		if got := ParseMap(form, map[string]any{"n": input}); len(got.Issues) != 1 || got.Issues[0].Code != CodeType {
			t.Fatalf("%v: %+v", input, got)
		}
	}
	for _, input := range []float64{-0x1p63, math.Nextafter(0x1p63, 0), 0} {
		got := ParseMap(form, map[string]any{"n": input})
		if len(got.Issues) != 0 || got.Values["n"] != int64(input) {
			t.Fatalf("%v: %+v", input, got)
		}
	}
}

func TestNativeTransportProvenanceAndCompatibility(t *testing.T) {
	token := EncodeEnumValue(false)
	form := model.FormModel{Fields: []model.Field{
		{Name: "text", Type: model.FieldTypeString, Enum: []any{token}, UIHints: map[string]string{"component": "input"}},
		{Name: "choice", Type: model.FieldTypeBoolean},
		{Name: "custom", Type: model.FieldTypeString, Metadata: map[string]string{"submission.enumValues": "control"}},
		{Name: "literal", Type: model.FieldTypeString, Enum: []any{token}, Metadata: map[string]string{"submission.enumValues": "literal"}},
		{Name: "codes", Type: model.FieldTypeArray, Metadata: map[string]string{"submission.enumValues": "control"}},
		{Name: "withItems", Type: model.FieldTypeArray, UIHints: map[string]string{"component": "select"}, Items: &model.Field{Type: model.FieldTypeString, Metadata: map[string]string{"host": "preserved"}}},
	}}
	raw := url.Values{"text": {token}, "choice": {token}, "custom": {EncodeEnumControlValue(token)}, "literal": {token}, "codes[]": {EncodeEnumControlValue(token)}, "withItems[]": {EncodeEnumControlValue(token)}}
	want := Values{"text": token, "choice": false, "custom": token, "literal": token, "codes": []any{token}, "withItems": []any{token}}
	before, _ := json.Marshal(form)
	got := ParseValues(form, raw)
	if !reflect.DeepEqual(got.Values, want) || len(got.Issues) != 0 {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
	after, _ := json.Marshal(form)
	if string(before) != string(after) {
		t.Fatal("parsing mutated model provenance")
	}
	// Explicit global transport modes retain their meaning for legacy hosts.
	legacy := ParseValues(form, raw, WithEnumValues(EnumControl))
	if legacy.Values["text"] != "false" {
		t.Fatal(legacy)
	}
	literal := ParseMap(form, map[string]any{"custom": token}, WithEnumValues(EnumLiteral))
	if literal.Values["custom"] != token {
		t.Fatal(literal)
	}
	auto := ParseMap(form, map[string]any{"custom": token})
	if auto.Values["custom"] != "false" {
		t.Fatal("legacy generic auto changed", auto)
	}
}

// The browser tests assert that actual FormData emits these exact entries.
func TestSharedNativeSubmissionPayloads(t *testing.T) {
	body, err := os.ReadFile("../../testdata/embedded/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		NativeSubmissionCases []struct {
			ID            string
			Form          model.FormModel
			NativeEntries [][2]string
			Values        Values
		}
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.NativeSubmissionCases) == 0 {
		t.Fatal("missing native payload fixtures")
	}
	for _, test := range fixture.NativeSubmissionCases {
		t.Run(test.ID, func(t *testing.T) {
			raw := url.Values{}
			for _, entry := range test.NativeEntries {
				raw.Add(entry[0], entry[1])
			}
			got := ParseValues(test.Form, raw)
			// JSON comparison normalizes native int64 versus fixture JSON numbers.
			actual, _ := json.Marshal(got.Values)
			expected, _ := json.Marshal(test.Values)
			if string(actual) != string(expected) || len(got.Issues) != 0 || len(Validate(test.Form, got.Values)) != 0 {
				t.Fatalf("got=%+v validate=%+v want=%s", got, Validate(test.Form, got.Values), expected)
			}
		})
	}
}
