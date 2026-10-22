package fieldpath

import (
	"encoding/json"
	"reflect"
	"testing"
)

type clonePayload struct{ Values []int }
type clonePointer *clonePayload
type cloneStrings []string
type cloneGroups map[string]cloneStrings

func TestClonePreservesConcreteJSONTypesAndOwnsContainers(t *testing.T) {
	original := map[string]any{
		"strings":    cloneStrings{"original"},
		"groups":     cloneGroups{"team": {"original"}},
		"array":      [1]map[string]any{{"codes": []string{"original"}}},
		"pointer":    clonePointer(&clonePayload{Values: []int{7}}),
		"raw":        json.RawMessage(`{"value":1}`),
		"large":      int64(9007199254740993),
		"lexeme":     json.Number("1.2300"),
		"nilMap":     map[string][]string(nil),
		"nilSlice":   []string(nil),
		"nilPointer": (*clonePayload)(nil),
		"nil":        nil,
	}
	copy := Clone(original).(map[string]any)
	if !reflect.DeepEqual(original, copy) {
		t.Fatalf("copy changed types or values: %#v", copy)
	}
	copy["strings"].(cloneStrings)[0] = "changed"
	copy["groups"].(cloneGroups)["team"][0] = "changed"
	copy["array"].([1]map[string]any)[0]["codes"].([]string)[0] = "changed"
	copy["pointer"].(clonePointer).Values[0] = 9
	copy["raw"].(json.RawMessage)[0] = '['
	if original["strings"].(cloneStrings)[0] != "original" || original["groups"].(cloneGroups)["team"][0] != "original" || original["array"].([1]map[string]any)[0]["codes"].([]string)[0] != "original" || original["pointer"].(clonePointer).Values[0] != 7 || original["raw"].(json.RawMessage)[0] != '{' {
		t.Fatalf("copy aliases input: %+v", original)
	}
}

func TestIndexesExistDoesNotRequireNonIndexedLeafPresence(t *testing.T) {
	values := map[string]any{"rows": []any{map[string]any{"nested": []any{nil}}}}
	for _, path := range []string{"object.answer", "rows[0].missing.answer", "rows[0].nested[0]"} {
		if !IndexesExist(values, path) {
			t.Fatalf("existing scope rejected: %s", path)
		}
	}
	for _, path := range []string{"rows[1].answer", "rows[0].missing[0]", "absent[0].answer", "rows[0].nested[1]"} {
		if IndexesExist(values, path) {
			t.Fatalf("nonexistent row accepted: %s", path)
		}
	}
}
