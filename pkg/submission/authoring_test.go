package submission_test

import (
	"encoding/json"
	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
	"os"
	"sort"
	"testing"
)

func TestSyntheticAuthoringContract(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/authoring.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Form  model.FormModel
		Cases []struct {
			ID     string
			Values map[string]any
			Issues []string
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if d := model.DiagnosePortableModel(fixture.Form); len(d) > 0 {
		t.Fatalf("preflight: %+v", d)
	}
	for _, c := range fixture.Cases {
		t.Run(c.ID, func(t *testing.T) {
			result := submission.ValidateConditional(fixture.Form, submission.ParseMap(fixture.Form, c.Values, submission.WithEnumValues(submission.EnumLiteral)), submission.InactiveReject)
			got := []string{}
			for _, issue := range result.Issues {
				got = append(got, string(issue.Code)+":"+issue.Path)
			}
			sort.Strings(got)
			sort.Strings(c.Issues)
			if len(got) != len(c.Issues) {
				t.Fatalf("got %v want %v", got, c.Issues)
			}
			for i := range got {
				if got[i] != c.Issues[i] {
					t.Fatalf("got %v want %v", got, c.Issues)
				}
			}
		})
	}
}
