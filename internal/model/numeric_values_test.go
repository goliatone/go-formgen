package model

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"
)

func TestToIntValueAcceptsLosslessJSONNumbers(t *testing.T) {
	type testCase struct {
		name  string
		value any
		want  int
		ok    bool
	}
	tests := []testCase{
		{name: "integer", value: json.Number("2"), want: 2, ok: true},
		{name: "integral decimal", value: json.Number("2.0"), want: 2, ok: true},
		{name: "fraction", value: json.Number("2.5"), ok: false},
		{name: "invalid", value: json.Number("invalid"), ok: false},
		{name: "positive infinity", value: math.Inf(1), ok: false},
		{name: "nan", value: math.NaN(), ok: false},
	}

	if strconv.IntSize == 64 {
		tests = append(tests,
			testCase{name: "maximum int", value: json.Number("9223372036854775807"), want: math.MaxInt, ok: true},
			testCase{name: "integer overflow", value: json.Number("9223372036854775808"), ok: false},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toIntValue(tt.value)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("toIntValue(%v) = (%d, %t), want (%d, %t)", tt.value, got, ok, tt.want, tt.ok)
			}
		})
	}
}
