package util

import (
	"encoding/json"
	"fmt"
)

// ToString attempts to coerce v into a string.
func ToString(v any) (string, bool) {
	s, ok := v.(string)
	if ok {
		return s, true
	}
	return "", false
}

// ToBool attempts to coerce v into a bool.
func ToBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	default:
		return false, false
	}
}

// ToInt attempts to coerce v into an int.
//
// When decoding JSON into map[string]any with json.Decoder.UseNumber(),
// numbers arrive as json.Number.
func ToInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

// ToInt64 attempts to coerce v into an int64.
//
// When decoding JSON into map[string]any with json.Decoder.UseNumber(),
// numbers arrive as json.Number.
func ToInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		return int64(x), true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

// MustJSON pretty-prints a JSON value for debugging/logging.
func MustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<json error: %v>", err)
	}
	return string(b)
}
