package model

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
)

var gridBreakpointKeys = map[string]struct{}{
	"sm":  {},
	"md":  {},
	"lg":  {},
	"xl":  {},
	"2xl": {},
}

func gridHintsFromExtensions(ext map[string]any) map[string]string {
	if len(ext) == 0 {
		return nil
	}

	grid := extractGridMap(ext)
	if len(grid) == 0 {
		return nil
	}

	out := make(map[string]string)
	applyGridValue(out, "layout.span", grid["span"])
	applyGridValue(out, "layout.start", grid["start"])
	applyGridValue(out, "layout.row", grid["row"])

	if breakpoints := toAnyMap(grid["breakpoints"]); len(breakpoints) > 0 {
		for key, value := range breakpoints {
			if _, ok := gridBreakpointKeys[key]; !ok {
				continue
			}
			if entry := toAnyMap(value); len(entry) > 0 {
				applyGridValue(out, "layout.span."+key, entry["span"])
				applyGridValue(out, "layout.start."+key, entry["start"])
				applyGridValue(out, "layout.row."+key, entry["row"])
			}
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func extractGridMap(ext map[string]any) map[string]any {
	if len(ext) == 0 {
		return nil
	}
	if raw, ok := ext[extensionNamespace]; ok {
		if nested := toAnyMap(raw); len(nested) > 0 {
			if grid := toAnyMap(nested["grid"]); len(grid) > 0 {
				return grid
			}
		}
	}
	if raw, ok := ext[extensionNamespace+"-grid"]; ok {
		if grid := toAnyMap(raw); len(grid) > 0 {
			return grid
		}
	}
	return nil
}

func applyGridValue(target map[string]string, key string, value any) {
	if target == nil || key == "" {
		return
	}
	num, ok := toIntValue(value)
	if !ok || num <= 0 {
		return
	}
	target[key] = strconv.Itoa(num)
}

func toIntValue(value any) (int, bool) {
	if number, ok := value.(json.Number); ok {
		if parsed, err := strconv.ParseInt(number.String(), 10, strconv.IntSize); err == nil {
			return int(parsed), true
		}
		parsed, err := number.Float64()
		if err != nil {
			return 0, false
		}
		return integralFloatToInt(parsed)
	}

	if str, ok := value.(string); ok {
		trimmed := strings.TrimSpace(str)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(trimmed)
		return parsed, err == nil
	}

	number := reflect.ValueOf(value)
	if !number.IsValid() {
		return 0, false
	}

	switch number.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		signed := number.Int()
		converted := int(signed)
		if int64(converted) != signed {
			return 0, false
		}
		return converted, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		unsigned := number.Uint()
		if unsigned > uint64(int(^uint(0)>>1)) {
			return 0, false
		}
		return int(unsigned), true
	case reflect.Float32, reflect.Float64:
		return integralFloatToInt(number.Float())
	default:
		return 0, false
	}
}

func integralFloatToInt(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, false
	}

	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	// The upper bound is exclusive because float64(maxInt) rounds to 2^63 on
	// 64-bit systems. The lower power-of-two bound remains exactly representable.
	if value < float64(minInt) || value >= -float64(minInt) {
		return 0, false
	}
	return int(value), true
}
