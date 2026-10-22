package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// DecodeConditions preserves the root extension as a typed authoritative contract.
func DecodeConditions(raw any) (*ConditionSet, error) {
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("model: invalid condition definition")
	}
	if len(payload) == 0 || payload[0] != '{' {
		return nil, fmt.Errorf("model: condition definition must be an object")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var set ConditionSet
	if err := decoder.Decode(&set); err != nil {
		return nil, fmt.Errorf("model: invalid condition definition shape")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("model: invalid condition definition")
	}
	return &set, nil
}
