package submission

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestValidatePathsPreciseAndIndependent(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject, Nested: []model.Field{{Name: "name", Type: model.FieldTypeString, Required: true}, {Name: "future", Type: model.FieldTypeString, Required: true}}}
	form := model.FormModel{Fields: []model.Field{{Name: "rows", Type: model.FieldTypeArray, Items: &item}, {Name: "future", Type: model.FieldTypeString, Required: true}}}
	values := Values{"rows": []any{map[string]any{"name": "ok"}, map[string]any{}}}
	got := ValidatePaths(form, values, []string{"rows[1].name"})
	if len(got) != 1 || got[0].Code != CodeRequired || got[0].Path != "rows[1].name" || got[0].RendererPath != "rows.name" {
		t.Fatalf("%+v", got)
	}
	if len(ValidatePaths(form, values, []string{"rows[0].name"})) != 0 {
		t.Fatal("unrelated required fields blocked scope")
	}
	if len(ValidatePaths(form, values, nil)) != 0 {
		t.Fatal("empty selection must validate nothing")
	}
	if len(ValidatePaths(form, values, []string{"rows[2].name"})) != 1 {
		t.Fatal("missing row answer must fail")
	}
	got = ValidatePaths(form, values, []string{"rows[1]", "rows[1].name", "rows[1]"})
	if len(got) != 2 {
		t.Fatalf("overlapping scopes duplicated issues: %+v", got)
	}
	for _, path := range []string{"rows[-1].name", "rows[01].name", "rows..name", "__proto__.x", "rows[].name"} {
		got = ValidatePaths(form, values, []string{path})
		if len(got) != 1 || got[0].Code != CodeInvalidPath {
			t.Fatalf("%s: %+v", path, got)
		}
	}
	got = ValidatePaths(form, values, []string{"unknown"})
	if len(got) != 1 || got[0].Code != CodeUnknownField {
		t.Fatal(got)
	}
}

func TestMissingArraysUnicodeAndTypedValues(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "choices", Type: model.FieldTypeArray, Required: true}, {Name: "enabled", Type: model.FieldTypeBoolean, Required: true}, {Name: "zero", Type: model.FieldTypeInteger, Required: true}, {Name: "text", Type: model.FieldTypeString, Validations: []model.ValidationRule{{Kind: model.ValidationRuleMaxLength, Params: map[string]string{"value": "2"}}}}}}
	got := Validate(form, Values{"enabled": false, "zero": int64(0), "text": "é😀"})
	if len(got) != 1 || got[0].Code != CodeRequired || got[0].Path != "choices" {
		t.Fatalf("%+v", got)
	}
	got = ValidatePaths(form, Values{"text": "é😀a"}, []string{"text"})
	if len(got) != 1 || got[0].Code != CodeMaxLength {
		t.Fatal(got)
	}
	got = ValidatePaths(form, Values{"text": "  "}, []string{"text"})
	if len(got) != 0 {
		t.Fatal(got)
	}
}
func TestUniqueItemsUsesTypedJSONEquality(t *testing.T) {
	field := model.Field{Name: "choices", Type: model.FieldTypeArray, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}}
	form := model.FormModel{Fields: []model.Field{field}}
	for _, items := range [][]any{{false, "false"}, {0, "0"}, {map[string]any{"x": false}, map[string]any{"x": "false"}}, {int64(9007199254740993), float64(9007199254740993)}, {math.NaN(), math.Inf(1)}} {
		if got := Validate(form, Values{"choices": items}); len(got) != 0 {
			t.Fatal(got)
		}
	}
	for _, items := range [][]any{{1, json.Number("1.0")}, {map[string]any{"x": 1}, map[string]any{"x": int64(1)}}, {[]string{"a"}, []any{"a"}}, {0.1, json.Number("0.1")}, {map[string]string{"k": "v"}, map[string]any{"k": "v"}}} {
		if got := Validate(form, Values{"choices": items}); len(got) != 1 || got[0].Code != CodeUniqueItems {
			t.Fatal(got)
		}
	}
}

type uniquenessPayload struct {
	Codes []string `json:"codes"`
}

// ParseMap accepts typed containers inside raw objects while ParseJSON decodes
// the identical JSON into untyped shapes. uniqueItems must report the same
// outcome for both, comparing by JSON structure and exact numeric value.
func TestUniqueItemsParityAcrossParseMapAndParseJSON(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject}
	form := model.FormModel{Fields: []model.Field{{Name: "answers", Type: model.FieldTypeArray, Items: &item, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}}}}
	for _, test := range []struct {
		name  string
		items []any
		want  int
	}{
		{"typed and untyped nested slices", []any{map[string]any{"codes": []string{"same"}}, map[string]any{"codes": []any{"same"}}}, 1},
		{"typed and untyped nested maps", []any{map[string]any{"meta": map[string]string{"k": "v"}}, map[string]any{"meta": map[string]any{"k": "v"}}}, 1},
		{"struct pointer and untyped object", []any{map[string]any{"meta": &uniquenessPayload{Codes: []string{"same"}}}, map[string]any{"meta": map[string]any{"codes": []any{"same"}}}}, 1},
		{"array and nested numeric lexemes", []any{map[string]any{"rows": [1][]int{{1, 2}}}, map[string]any{"rows": []any{[]any{json.Number("1"), 2.0}}}}, 1},
		{"float64 and json.Number decimal", []any{map[string]any{"score": 0.1}, map[string]any{"score": json.Number("0.1")}}, 1},
		{"int64 and json.Number beyond 2^53", []any{map[string]any{"n": int64(9007199254740993)}, map[string]any{"n": json.Number("9007199254740993")}}, 1},
		{"uint64 maximum and json.Number", []any{map[string]any{"n": uint64(math.MaxUint64)}, map[string]any{"n": json.Number("18446744073709551615")}}, 1},
		{"nil typed slice and null", []any{map[string]any{"codes": []string(nil)}, map[string]any{"codes": nil}}, 1},
		{"adjacent large integers stay distinct", []any{map[string]any{"n": int64(9007199254740993)}, map[string]any{"n": int64(9007199254740992)}}, 0},
		{"rounded float is not the large integer", []any{map[string]any{"n": float64(9007199254740993)}, map[string]any{"n": json.Number("9007199254740993")}}, 0},
		{"distinct typed slices stay distinct", []any{map[string]any{"codes": []string{"a"}}, map[string]any{"codes": []any{"b"}}}, 0},
		{"explicit null and missing key stay distinct", []any{map[string]any{"x": nil}, map[string]any{}}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := map[string]any{"answers": test.items}
			fromMap := ParseMap(form, raw)
			body, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			fromJSON, err := ParseJSON(form, body)
			if err != nil {
				t.Fatal(err)
			}
			if len(fromMap.Issues) != 0 || len(fromJSON.Issues) != 0 {
				t.Fatalf("parse issues: map=%+v json=%+v", fromMap.Issues, fromJSON.Issues)
			}
			got := Validate(form, fromMap.Values)
			if viaJSON := Validate(form, fromJSON.Values); !reflect.DeepEqual(got, viaJSON) {
				t.Fatalf("ParseMap/ParseJSON disagree: map=%+v json=%+v", got, viaJSON)
			}
			if scoped := ValidatePaths(form, fromMap.Values, []string{"answers"}); !reflect.DeepEqual(got, scoped) {
				t.Fatalf("scoped validation disagrees: full=%+v scoped=%+v", got, scoped)
			}
			if len(got) != test.want {
				t.Fatalf("want %d uniqueItems issues, got %+v", test.want, got)
			}
			if test.want == 1 && (got[0].Code != CodeUniqueItems || got[0].Path != "answers") {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestScopePreservesParsedValuesAndIssues(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "zero", Type: model.FieldTypeInteger}, {Name: "future", Type: model.FieldTypeBoolean}}}
	parsed := ParseMap(form, map[string]any{"zero": "0", "future": "bad"})
	before := append([]Issue{}, parsed.Issues...)
	if got := ValidatePaths(form, parsed.Values, []string{"zero"}); len(got) != 0 {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(before, parsed.Issues) || parsed.Values["zero"] != int64(0) {
		t.Fatal("scope mutated parse result")
	}
}
func TestEnumPrefixAndStringFalseRemainDistinct(t *testing.T) {
	prefix := EncodeEnumValue(false)
	form := model.FormModel{Fields: []model.Field{{Name: "code", Type: model.FieldTypeString, Enum: []any{prefix, "false"}}}}
	for _, value := range []string{prefix, "false"} {
		parsed := ParseMap(form, map[string]any{"code": EncodeEnumControlValue(value)})
		if len(parsed.Issues) != 0 || parsed.Values["code"] != value || len(Validate(form, parsed.Values)) != 0 {
			t.Fatalf("%+v", parsed)
		}
	}
}

func TestIndexedScopePreservesParentAndItemEnums(t *testing.T) {
	for _, test := range []struct {
		name, value string
		want        int
	}{
		{"parent rejects", "red", 1}, {"item rejects", "green", 1}, {"both reject", "yellow", 2}, {"intersection accepts", "blue", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			form := model.FormModel{Fields: []model.Field{{Name: "colors", Type: model.FieldTypeArray, Enum: []any{"blue", "green"}, Items: &model.Field{Type: model.FieldTypeString, Enum: []any{"blue", "red"}}}}}
			values := Values{"colors": []any{test.value, "blue"}}
			full := Validate(form, values)
			scoped := ValidatePaths(form, values, []string{"colors[0]"})
			if len(scoped) != test.want || !reflect.DeepEqual(scoped, full) {
				t.Fatalf("scoped=%+v full=%+v want=%d", scoped, full, test.want)
			}
		})
	}
}

func TestIndexedScopeOnlyInheritsImmediateElementConstraints(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "matrix", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeArray, Enum: []any{"blue"}, Items: &model.Field{Type: model.FieldTypeString}}}}}
	values := Values{"matrix": []any{[]any{"red", "blue"}}}
	got := ValidatePaths(form, values, []string{"matrix[0][0]"})
	if len(got) != 1 || got[0].Code != CodeEnum || got[0].Path != "matrix[0][0]" {
		t.Fatal(got)
	}
	// A child selection does not validate unrelated ancestors, siblings or array counts.
	form.Fields[0].Items.Enum = nil
	form.Fields[0].Items.Validations = []model.ValidationRule{{Kind: model.ValidationRuleMinItems, Params: map[string]string{"value": "3"}}}
	if got := ValidatePaths(form, values, []string{"matrix[0][0]"}); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestCanonicalNumberSharesOneLexemePerValue(t *testing.T) {
	for _, test := range []struct{ lexeme, want string }{
		{"100", "1e2"}, {"1e2", "1e2"}, {"1E+2", "1e2"}, {"10e1", "1e2"}, {"1.0", "1e0"}, {"1", "1e0"},
		{"0.10", "1e-1"}, {"0.1", "1e-1"}, {"1e-1", "1e-1"}, {"-1.50", "-15e-1"}, {"-0", "0"}, {"0.0", "0"}, {"0e5", "0"},
		{"9007199254740993", "9007199254740993e0"}, {"18446744073709551615", "18446744073709551615e0"},
		{"1e999999", "1e999999"}, {"10e999998", "1e999999"}, {"1e99999999999", "1e99999999999"}, {"abc", "abc"},
		{"1.0e2147483648", "1e2147483648"}, {"10e9223372036854775807", "1e9223372036854775808"},
		{"0.1e-9223372036854775808", "1e-9223372036854775809"}, {"-0e9223372036854775808", "0"},
		{"1e+0002147483648", "1e2147483648"},
		{"10e" + strings.Repeat("9", 1024), "1e1" + strings.Repeat("0", 1024)},
		{"10e-1" + strings.Repeat("0", 1024), "1e-" + strings.Repeat("9", 1024)},
	} {
		if got := canonicalNumber(json.Number(test.lexeme)); string(got) != test.want {
			t.Fatalf("%s: want %s, got %s", test.lexeme, test.want, got)
		}
	}
}

// Exponents remain decimal text: equivalent raw-object answers must be rejected
// by every validation entry point without constructing their numeric magnitude.
func TestUniqueItemsArbitraryExponentsAcrossValidationPipeline(t *testing.T) {
	form := model.FormModel{
		Fields: []model.Field{
			{Name: "enabled", Type: model.FieldTypeBoolean},
			{Name: "answers", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeObject}, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}},
		},
		Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "answers", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}}},
	}
	for _, test := range []struct {
		name, first, second string
		duplicates          bool
	}{
		{"beyond int32", "1.0e2147483648", "1e2147483648", true},
		{"beyond int64", "10e9223372036854775807", "1e9223372036854775808", true},
		{"negative exponent", "0.1e-9223372036854775808", "1e-9223372036854775809", true},
		{"zero", "-0e9223372036854775808", "0", true},
		{"long exponent", "10e" + strings.Repeat("9", 1024), "1e1" + strings.Repeat("0", 1024), true},
		{"adjacent exponents", "1e9223372036854775808", "1e9223372036854775809", false},
		{"opposite signs", "1e9223372036854775808", "-1e9223372036854775808", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := map[string]any{"enabled": true, "answers": []any{map[string]any{"n": json.Number(test.first)}, map[string]any{"n": json.Number(test.second)}}}
			body, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := ParseJSON(form, body)
			if err != nil {
				t.Fatal(err)
			}
			for _, parsed := range []Result{ParseMap(form, raw), wire} {
				if len(parsed.Issues) != 0 {
					t.Fatal(parsed.Issues)
				}
				want := 0
				if test.duplicates {
					want = 1
				}
				full := Validate(form, parsed.Values)
				if len(full) != want || (want == 1 && (full[0].Code != CodeUniqueItems || full[0].Path != "answers")) {
					t.Fatalf("want %d uniqueness issues, got %+v", want, full)
				}
				if scoped := ValidatePaths(form, parsed.Values, []string{"answers"}); !reflect.DeepEqual(scoped, full) {
					t.Fatalf("scoped=%+v full=%+v", scoped, full)
				}
				if conditional := ValidateConditional(form, parsed, InactiveReject); !reflect.DeepEqual(conditional.Issues, full) {
					t.Fatalf("conditional=%+v full=%+v", conditional.Issues, full)
				}
				parsed.Values["enabled"] = false
				if inactive := ValidateConditional(form, parsed, InactiveNormalize); len(inactive.Issues) != 0 {
					t.Fatalf("inactive duplicates must be excluded: %+v", inactive.Issues)
				}
			}
		})
	}
}

// Untrusted JSON can carry numeric lexemes with huge exponents; uniqueness must
// not expand their numeric magnitude or compare every pair.
func TestUniqueItemsStaysLinearForLargeExponentLexemes(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject}
	form := model.FormModel{Fields: []model.Field{{Name: "answers", Type: model.FieldTypeArray, Items: &item, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}}}}
	var body strings.Builder
	body.WriteString(`{"answers":[`)
	for i := range 400 {
		if i > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(&body, `{"n":1e%d,"m":%d.5}`, 999000+i, i)
	}
	body.WriteString("]}")
	parsed, err := ParseJSON(form, []byte(body.String()))
	if err != nil || len(parsed.Issues) != 0 {
		t.Fatalf("%v %+v", err, parsed.Issues)
	}
	start := time.Now()
	if got := Validate(form, parsed.Values); len(got) != 0 {
		t.Fatal(got)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("validation took %s", elapsed)
	}
	parsed, _ = ParseJSON(form, []byte(`{"answers":[{"n":1e999999},{"n":10e999998}]}`))
	if got := Validate(form, parsed.Values); len(got) != 1 || got[0].Code != CodeUniqueItems {
		t.Fatal(got)
	}
}

// Host-built values can hold containers encoding/json rejects; they must
// neither crash nor recurse without bound.
func TestUniqueItemsToleratesCyclicAndUnencodableValues(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "choices", Type: model.FieldTypeArray, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}}}}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	loop := []any{nil}
	loop[0] = loop
	for _, items := range [][]any{{cyclic, cyclic}, {loop, loop}, {cyclic, loop, cyclic}} {
		if got := Validate(form, Values{"choices": items}); len(got) != 1 || got[0].Code != CodeUniqueItems {
			t.Fatalf("same object twice must be a duplicate: %+v", got)
		}
	}
	for _, items := range [][]any{{cyclic, map[string]any{"self": nil}}, {cyclic, loop}, {math.NaN(), math.NaN()}, {json.Number("abc"), "abc"}} {
		if got := Validate(form, Values{"choices": items}); len(got) != 0 {
			t.Fatal(got)
		}
	}
}

type typedCodes []int

// Typed array fields must parse host-supplied typed slices, arrays and
// pointers like the []any shape ParseJSON produces instead of folding them
// into one stringified item.
func TestTypedContainersInNestedObjectsParseLikeJSON(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject, Nested: []model.Field{{Name: "tags", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeInteger}}, {Name: "name", Type: model.FieldTypeString}}}
	form := model.FormModel{Fields: []model.Field{{Name: "rows", Type: model.FieldTypeArray, Items: &item, Validations: []model.ValidationRule{{Kind: model.ValidationRuleUniqueItems}}}}}
	raw := map[string]any{"rows": []any{
		map[string]any{"tags": []int{1, 2}, "name": "n"},
		map[string]any{"tags": typedCodes{1, 2}, "name": "n"},
		map[string]any{"tags": &[2]int64{1, 2}, "name": "n"},
		map[string]any{"tags": []any{1, 2}, "name": "n"},
	}}
	fromMap := ParseMap(form, raw)
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	fromJSON, err := ParseJSON(form, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromMap.Issues) != 0 || len(fromJSON.Issues) != 0 || !reflect.DeepEqual(fromMap.Values, fromJSON.Values) {
		t.Fatalf("map=%+v %+v json=%+v %+v", fromMap.Values, fromMap.Issues, fromJSON.Values, fromJSON.Issues)
	}
	for i, row := range fromMap.Values["rows"].([]any) {
		if tags := row.(map[string]any)["tags"]; !reflect.DeepEqual(tags, []any{int64(1), int64(2)}) {
			t.Fatalf("row %d tags: %#v", i, tags)
		}
	}
	got := Validate(form, fromMap.Values)
	if len(got) != 1 || got[0].Code != CodeUniqueItems || got[0].Path != "rows" || !reflect.DeepEqual(got, Validate(form, fromJSON.Values)) {
		t.Fatal(got)
	}
	parsed := ParseMap(form, map[string]any{"rows": []any{map[string]any{"tags": []int(nil), "name": "n"}, map[string]any{"tags": (*[]int)(nil), "name": "n"}, map[string]any{"tags": []any(nil), "name": "n"}, map[string]any{"tags": []string(nil), "name": "n"}, map[string]any{"tags": 7, "name": "n"}, map[string]any{"tags": [3]byte{1, 2, 3}, "name": "n"}}})
	if len(parsed.Issues) != 0 {
		t.Fatal(parsed.Issues)
	}
	rows := parsed.Values["rows"].([]any)
	for i := range 4 {
		if tags, exists := rows[i].(map[string]any)["tags"]; !exists || tags != nil {
			t.Fatalf("row %d: nil containers must parse as null: %#v", i, tags)
		}
	}
	if tags := rows[4].(map[string]any)["tags"]; !reflect.DeepEqual(tags, []any{int64(7)}) {
		t.Fatalf("scalar must stay a single item: %#v", tags)
	}
	if tags := rows[5].(map[string]any)["tags"]; !reflect.DeepEqual(tags, []any{int64(1), int64(2), int64(3)}) {
		t.Fatalf("byte arrays encode as JSON arrays: %#v", tags)
	}
}

func TestReservedPrefixLiteralChoiceRoundTrip(t *testing.T) {
	literal := EncodeEnumValue(false)
	form := model.FormModel{Fields: []model.Field{{Name: "code", Type: model.FieldTypeString, Enum: []any{literal, "false"}}, {Name: "codes", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeString, Enum: []any{literal, "false"}}}}}
	for _, value := range []string{literal, EncodeEnumControlValue(literal)} {
		parsed := ParseMap(form, map[string]any{"code": value, "codes": []any{value, "false"}})
		if len(parsed.Issues) > 0 || parsed.Values["code"] != literal || parsed.Values["codes"].([]any)[0] != literal {
			t.Fatalf("%+v", parsed)
		}
		if issues := Validate(form, parsed.Values); len(issues) > 0 {
			t.Fatal(issues)
		}
	}
}

func TestReservedPrefixScalarArrayInput(t *testing.T) {
	literal := EncodeEnumValue(false)
	form := model.FormModel{Fields: []model.Field{{Name: "codes", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeString, Enum: []any{literal}}}}}
	got := ParseMap(form, map[string]any{"codes": literal})
	if len(got.Issues) > 0 || got.Values["codes"].([]any)[0] != literal {
		t.Fatalf("%+v", got)
	}
}
