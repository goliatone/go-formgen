package model_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestEmbeddedContractFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string
		Form    model.FormModel
		Cases   []json.RawMessage
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "formgen.embedded/v1" || fixture.Form.Conditions.Version != model.ConditionsVersion || len(fixture.Cases) < 9 {
		t.Fatal("unsupported contract fixture")
	}
	for _, value := range []any{false, 0, "false"} {
		set := model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "rows[2].answer", ApplicableWhen: &model.Expression{Op: "eq", Path: "rows[2].enabled", Value: value}}}}
		data, err := json.Marshal(set)
		if err != nil {
			t.Fatal(err)
		}
		var got model.ConditionSet
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Rules[0].ApplicableWhen.Value == nil {
			t.Fatalf("lost present value %v", value)
		}
	}
}
