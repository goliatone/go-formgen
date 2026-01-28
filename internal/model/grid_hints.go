package model

import (
	"math"
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
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float64:
		if v == math.Trunc(v) {
			return int(v), true
		}
	case float32:
		if float64(v) == math.Trunc(float64(v)) {
			return int(v), true
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		value, err := strconv.Atoi(trimmed)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}
