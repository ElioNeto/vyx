//go:build with_aws
// +build with_aws

package aws

import (
	"encoding/json"
	"fmt"
)

// valuesEqual compares two values with JSON type normalization.
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	af, aOk := toFloat64(a)
	bf, bOk := toFloat64(b)
	if aOk && bOk {
		return af == bf
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint8:
		return float64(val), true
	case json.Number:
		f, _ := val.Float64()
		return f, true
	default:
		return 0, false
	}
}

// getStringProp retrieves a string property with a default fallback.
func getStringProp(props map[string]any, key, defaultVal string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getBoolProp retrieves a boolean property with a default fallback.
func getBoolProp(props map[string]any, key string, defaultVal bool) bool {
	if v, ok := props[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return val == "true"
		}
	}
	return defaultVal
}

// getIntProp retrieves an integer property with a default fallback.
func getIntProp(props map[string]any, key string, defaultVal int) int {
	if v, ok := props[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case int64:
			return int(val)
		case json.Number:
			i, _ := val.Int64()
			return int(i)
		}
	}
	return defaultVal
}
